/* ###
 * Lemonade Tycoon 2 Borland/CodeGear C++ Builder demangler shim.
 *
 * This is intentionally conservative.  It handles the known operator
 * encodings seen in the Lemonade Tycoon 2 Borland CRT and gives @-separated Borland symbols
 * stable, readable Ghidra-safe names while preserving the raw spelling in the
 * function plate comment.
 */
//@category Lemonade Tycoon 2

import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.FunctionIterator;
import com.lemonadetycoon.ghidra.AnalysisScriptSupport;
import com.google.gson.JsonObject;
import java.io.File;
import java.util.HashMap;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class BorlandDemangler extends AnalysisScriptSupport {

	private static final Pattern BORLAND_UNDERSCORE =
		Pattern.compile("^__([0-9]+)_([A-Za-z0-9_@]+)_Z$");
	private static final Pattern BORLAND_AT = Pattern.compile("^@[^\\s]+");
	private static final String DEFAULT_LOG = "decompiled/analysis/borland_demangler.jsonl";
	private static final String EVENT_SCHEMA = "lt2.analysis_event.v1";
	private static final String STAGE = "borland_demangler";
	private final Map<String, DemangleResult> exact = new HashMap<String, DemangleResult>();

	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("BorlandDemangler: no current program");
			return;
		}
		initExact();
		int before = countDefaultFunctions();
		int renamed = 0;
		int candidates = 0;
			File log = resolveProjectLogFile("lemonade.borland.log", "lt2.borland.log",
				DEFAULT_LOG);
			ensureFreshLogFile(log);

		int transaction = currentProgram.startTransaction("Lemonade Tycoon 2 Borland demangle");
		try {
			FunctionIterator functions =
				currentProgram.getFunctionManager().getFunctions(true);
			while (functions.hasNext() && !monitor.isCancelled()) {
				Function function = functions.next();
				String raw = function.getName();
				DemangleResult result = demangle(raw);
				if (result == null) {
					continue;
				}
				candidates++;
				String oldName = raw;
				String newName = renameWithAddressFallback(function, result.safeName,
					"BorlandDemangler");
				if (newName != null) {
					setPlateComment(function.getEntryPoint(),
						"Borland demangled: " + result.display + "\nMangled: " + oldName);
					renamed++;
					appendLog(log, function, oldName, newName, result);
				}
			}
		}
		finally {
			currentProgram.endTransaction(transaction, true);
		}

		int after = countDefaultFunctions();
		println("BorlandDemangler: " + candidates + " candidate(s), " + renamed +
			" rename(s), FUN_* " + before + " -> " + after);
		println("BorlandDemangler self-test: __3_YAXPAX_Z -> " +
			demangle("__3_YAXPAX_Z").display);
	}

	private void initExact() {
		if (!exact.isEmpty()) {
			return;
		}
		exact.put("__3_YAXPAX_Z", new DemangleResult(
			"Borland_operator_delete_void_ptr", "operator delete(void *)", 98));
		exact.put("__2_YAPAXI_Z", new DemangleResult(
			"Borland_operator_new_uint", "operator new(unsigned int)", 98));
		exact.put("__3_YAXPAXI_Z", new DemangleResult(
			"Borland_operator_delete_array_void_ptr_uint",
			"operator delete[](void *, unsigned int)", 90));
		exact.put("__2_YAPAXI@Z", new DemangleResult(
			"Borland_operator_new_uint_alt", "operator new(unsigned int)", 85));
	}

	private DemangleResult demangle(String raw) {
		DemangleResult exactResult = exact.get(raw);
		if (exactResult != null) {
			return exactResult;
		}

		Matcher underscore = BORLAND_UNDERSCORE.matcher(raw);
		if (underscore.matches()) {
			String safe = "Borland_mangled_" + underscore.group(1) + "_" +
				safeIdentifier(underscore.group(2));
			return new DemangleResult(safe, raw + " (Borland encoded)", 45);
		}

		if (!BORLAND_AT.matcher(raw).matches()) {
			return null;
		}
		String body = raw;
		while (body.startsWith("@")) {
			body = body.substring(1);
		}
		int sig = body.indexOf('$');
		String qualified = sig >= 0 ? body.substring(0, sig) : body;
		if (qualified.length() == 0) {
			return null;
		}
		String display = qualified.replace('@', ':').replace(":::", "::");
		String safe = "Borland_" + safeIdentifier(qualified.replace('@', '_'));
		return new DemangleResult(safe, display, 70);
	}

	private void appendLog(File file, Function function, String oldName, String newName,
			DemangleResult result) throws Exception {
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
		event.addProperty("category", "borland_demangle");
		event.addProperty("confidence", result.confidence);
		event.addProperty("decision", "accepted");
		event.addProperty("evidence", result.display);
		appendJsonLine(file, event);
	}

	private static class DemangleResult {
		String safeName;
		String display;
		int confidence;

		DemangleResult(String safeName, String display, int confidence) {
			this.safeName = safeName;
			this.display = display;
			this.confidence = confidence;
		}
	}
}
