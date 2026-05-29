/* ###
 * Lemonade Tycoon 2 import thunk resolver.
 */
//@category Lemonade Tycoon 2

import ghidra.app.util.NamespaceUtils;
import ghidra.program.model.address.Address;
import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.FunctionIterator;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.listing.InstructionIterator;
import ghidra.program.model.symbol.Namespace;
import ghidra.program.model.symbol.Reference;
import ghidra.program.model.symbol.Symbol;
import com.lemonadetycoon.ghidra.AnalysisScriptSupport;
import com.google.gson.JsonObject;
import java.io.File;

public class ResolveImportThunks extends AnalysisScriptSupport {

	private static final String DEFAULT_LOG = "decompiled/analysis/resolved_iat_thunks.jsonl";
	private static final String EVENT_SCHEMA = "lt2.analysis_event.v1";
	private static final String STAGE = "iat_thunk_resolver";

	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("ResolveImportThunks: no current program");
			return;
		}
		int before = countDefaultFunctions();
			int considered = 0;
			int renamed = 0;
			File log = resolveProjectLogFile("lemonade.iat.log", "lt2.iat.log", DEFAULT_LOG);
			ensureFreshLogFile(log);

		int transaction = currentProgram.startTransaction("Lemonade Tycoon 2 resolve IAT thunks");
		try {
			FunctionIterator functions =
				currentProgram.getFunctionManager().getFunctions(true);
			while (functions.hasNext() && !monitor.isCancelled()) {
				Function function = functions.next();
				if (!function.getName().startsWith("FUN_") && !function.isThunk()) {
					continue;
				}
				considered++;
				ThunkResult target = resolveThunk(function);
				if (target == null) {
					continue;
				}
				String oldName = function.getName();
				String newName = renameWithAddressFallback(function, target.name,
					"ResolveImportThunks");
				if (newName == null) {
					continue;
				}
				setPlateComment(function.getEntryPoint(),
					"Lemonade Tycoon 2 auto-resolved import thunk: " + target.display);
				appendLog(log, function, oldName, newName, target);
				renamed++;
			}
		}
		finally {
			currentProgram.endTransaction(transaction, true);
		}

		int after = countDefaultFunctions();
		println("ResolveImportThunks: considered " + considered +
			" thunk candidate(s), renamed " + renamed + ", FUN_* " + before +
			" -> " + after);
	}

	private ThunkResult resolveThunk(Function function) {
		try {
			Function thunked = function.getThunkedFunction(true);
			if (thunked != null && thunked != function &&
				!thunked.getName().startsWith("FUN_")) {
				return new ThunkResult(importName(thunked.getSymbol()), thunked.getName());
			}
		}
		catch (Exception ignored) {
		}

		InstructionIterator instructions =
			currentProgram.getListing().getInstructions(function.getBody(), true);
		if (!instructions.hasNext()) {
			return null;
		}
		Instruction first = instructions.next();
		if (instructions.hasNext() ||
			!"JMP".equalsIgnoreCase(first.getMnemonicString())) {
			return null;
		}

		Reference[] refs = first.getReferencesFrom();
		for (int i = 0; i < refs.length; i++) {
			ThunkResult result = resolveTarget(refs[i].getToAddress());
			if (result != null) {
				return result;
			}
		}
		for (int op = 0; op < first.getNumOperands(); op++) {
			Reference[] opRefs = first.getOperandReferences(op);
			for (int i = 0; i < opRefs.length; i++) {
				ThunkResult result = resolveTarget(opRefs[i].getToAddress());
				if (result != null) {
					return result;
				}
			}
		}
		return null;
	}

	private ThunkResult resolveTarget(Address address) {
		Symbol symbol = getSymbolAt(address);
		if (usableImportSymbol(symbol)) {
			String display = namespacePath(symbol) + symbol.getName();
			return new ThunkResult(importName(symbol), display);
		}
		try {
			if (currentProgram.getMemory().contains(address)) {
				long pointer = Integer.toUnsignedLong(currentProgram.getMemory().getInt(address));
				Address pointed = currentProgram.getAddressFactory()
					.getDefaultAddressSpace().getAddress(pointer);
				symbol = getSymbolAt(pointed);
				if (usableImportSymbol(symbol)) {
					String display = namespacePath(symbol) + symbol.getName();
					return new ThunkResult(importName(symbol), display);
				}
			}
		}
		catch (Exception ignored) {
		}
		return null;
	}

	private boolean usableImportSymbol(Symbol symbol) {
		if (symbol == null) {
			return false;
		}
		String name = symbol.getName();
		return name != null && name.length() > 0 &&
			!name.startsWith("FUN_") && !name.startsWith("DAT_") &&
			!name.startsWith("LAB_");
	}

	private String importName(Symbol symbol) {
		String name = symbol.getName();
		if (name.startsWith("__imp_")) {
			name = name.substring("__imp_".length());
		}
		String ns = namespacePath(symbol).replace("::", "_");
		return safeIdentifier("IAT_" + ns + name);
	}

	private String namespacePath(Symbol symbol) {
		Namespace ns = symbol.getParentNamespace();
		if (ns == null || ns.isGlobal()) {
			return "";
		}
		String path = NamespaceUtils.getNamespacePathWithoutLibrary(ns);
		return path == null || path.length() == 0 ? "" : path + "::";
	}

	private void appendLog(File file, Function function, String oldName, String newName,
			ThunkResult target) throws Exception {
		JsonObject event = new JsonObject();
		event.addProperty("schema", EVENT_SCHEMA);
		event.addProperty("stage", STAGE);
		event.addProperty("program", currentProgram.getName());
		event.addProperty("executable_md5", currentProgram.getExecutableMD5());
		event.addProperty("address", function.getEntryPoint().toString());
		event.addProperty("action", "renamed");
		event.addProperty("symbol_name", newName);
		event.addProperty("old_name", oldName);
		event.addProperty("new_name", newName);
		event.addProperty("category", "iat_thunk");
		event.addProperty("confidence", 95);
		event.addProperty("decision", "accepted");
		event.addProperty("evidence", target.display);
		appendJsonLine(file, event);
	}

	private static class ThunkResult {
		String name;
		String display;

		ThunkResult(String name, String display) {
			this.name = name;
			this.display = display;
		}
	}
}
