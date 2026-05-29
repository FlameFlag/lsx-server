package com.lemonadetycoon.ghidra;

import ghidra.app.script.GhidraScript;
import ghidra.program.model.address.Address;
import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.FunctionIterator;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.mem.Memory;
import ghidra.program.model.symbol.FlowType;
import ghidra.program.model.symbol.SourceType;
import ghidra.program.model.symbol.SymbolUtilities;
import ghidra.util.StringUtilities;
import ghidra.util.exception.DuplicateNameException;
import ghidra.util.exception.InvalidInputException;
import com.lemonadetycoon.ghidra.analysis.BadFlow;
import com.google.gson.Gson;
import com.google.gson.JsonObject;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.StandardOpenOption;
import java.util.HexFormat;
import java.util.Locale;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.regex.Pattern;
import org.apache.commons.lang3.StringUtils;

public abstract class AnalysisScriptSupport extends GhidraScript {
	private static final Gson GSON = new Gson();
	private static final HexFormat SPACE_SEPARATED_HEX = HexFormat.ofDelimiter(" ");
	private static final Pattern UNSAFE_IDENTIFIER_CHARS = Pattern.compile("[^A-Za-z0-9_]");
	private static final Pattern REPEATED_UNDERSCORES = Pattern.compile("_+");
	private static final Set<String> initializedLogs = ConcurrentHashMap.newKeySet();

	protected int countDefaultFunctions() {
		int count = 0;
		FunctionIterator functions = currentProgram.getFunctionManager().getFunctions(true);
		while (functions.hasNext()) {
			if (functions.next().getName().startsWith("FUN_")) {
				count++;
			}
		}
		return count;
	}

	protected File resolveProjectLogFile(String propertyName, String defaultPath) {
		return resolveProjectLogFile(propertyName, null, defaultPath);
	}

	protected File resolveProjectLogFile(String propertyName, String legacyPropertyName,
			String defaultPath) {
		String override = System.getProperty(propertyName);
		if (override == null && legacyPropertyName != null) {
			override = System.getProperty(legacyPropertyName);
		}
		if (override == null) {
			override = defaultPath;
		}
		File file = new File(override);
		if (!file.isAbsolute()) {
			file = new File(System.getProperty("user.dir"), override);
		}
		File parent = file.getParentFile();
		if (parent != null) {
			parent.mkdirs();
		}
		return file;
	}

	protected void ensureFreshLogFile(File file) throws Exception {
		String key = file.getAbsolutePath();
		if (initializedLogs.contains(key) && file.isFile()) {
			return;
		}
		initializedLogs.add(key);
		Files.writeString(file.toPath(), "", StandardCharsets.UTF_8,
			StandardOpenOption.CREATE, StandardOpenOption.TRUNCATE_EXISTING);
	}

	protected void appendJsonLine(File file, JsonObject object) throws Exception {
		appendLine(file, GSON.toJson(object));
	}

	protected void appendLine(File file, String line) throws Exception {
		Files.writeString(file.toPath(), line + "\n", StandardCharsets.UTF_8,
			StandardOpenOption.CREATE, StandardOpenOption.APPEND);
	}

	protected String renameWithAddressFallback(Function function, String base, String logPrefix) {
		try {
			function.setName(base, SourceType.ANALYSIS);
			return base;
		}
		catch (DuplicateNameException | InvalidInputException duplicateOrInvalid) {
			String name = nameWithEntryAddress(base, function);
			try {
				function.setName(name, SourceType.ANALYSIS);
				return name;
			}
			catch (DuplicateNameException | InvalidInputException e) {
				String autoName = name + "_auto";
				try {
					function.setName(autoName, SourceType.ANALYSIS);
					return autoName;
				}
				catch (DuplicateNameException | InvalidInputException autoException) {
					println(logPrefix + ": could not rename " + function.getEntryPoint() +
						" to " + autoName + ": " + autoException.getMessage());
					return null;
				}
			}
		}
	}

	protected String nameWithEntryAddress(String base, Function function) {
		String name = SymbolUtilities.getAddressAppendedName(base, function.getEntryPoint());
		if (base.equals(name) || base.endsWith("_" + function.getEntryPoint())) {
			return base;
		}
		return name;
	}

	protected String safeIdentifier(String text) {
		String safe = SymbolUtilities.replaceInvalidChars(text, true);
		safe = UNSAFE_IDENTIFIER_CHARS.matcher(safe).replaceAll("_");
		safe = REPEATED_UNDERSCORES.matcher(safe).replaceAll("_");
		if (safe.length() == 0) {
			return "symbol";
		}
		if (Character.isDigit(safe.charAt(0))) {
			return "_" + safe;
		}
		return safe;
	}

	protected String bytesToHex(byte[] bytes) {
		return SPACE_SEPARATED_HEX.formatHex(bytes);
	}

	protected String repeatedByteHex(int count, String value) {
		return StringUtils.repeat(value, " ", count);
	}

	protected String directBadFlowReason(Instruction instruction) {
		BadFlow badFlow = directBadFlow(instruction);
		return badFlow == null ? null : badFlow.reason();
	}

	protected BadFlow directBadFlow(Instruction instruction) {
		FlowType flowType = instruction.getFlowType();
		if (!flowType.isJump() && !flowType.isCall() && !flowType.isConditional()) {
			return null;
		}
		if (!isDirectFlow(instruction)) {
			return null;
		}

		Address[] flows = instruction.getDefaultFlows();
		if (flows.length == 0) {
			return null;
		}
		Memory memory = currentProgram.getMemory();
		for (Address target : flows) {
			if (target == null || !target.isMemoryAddress()) {
				continue;
			}
			if (!memory.contains(target)) {
				return new BadFlow(target, "direct flow to unmapped memory");
			}
			Instruction containing = getInstructionContaining(target);
			if (containing != null && !containing.getMinAddress().equals(target)) {
				return new BadFlow(target, "direct flow into middle of instruction at " +
					containing.getMinAddress());
			}
			Instruction previous = getInstructionBefore(target);
			if (previous != null && previous.getMaxAddress().compareTo(target) >= 0) {
				return new BadFlow(target, "direct flow to overlapping offcut instruction after " +
					previous.getMinAddress());
			}
		}
		return null;
	}

	protected boolean isDirectFlow(Instruction instruction) {
		String text = instruction.toString().toLowerCase(Locale.ROOT);
		if (text.contains("[") || text.contains("ptr")) {
			return false;
		}
		String mnemonic = instruction.getMnemonicString().toUpperCase(Locale.ROOT);
		return mnemonic.startsWith("J") || mnemonic.equals("CALL");
	}

	protected boolean containsAll(String text, String... needles) {
		return StringUtilities.containsAll(text, needles);
	}

	protected int countOccurrences(String text, String needle) {
		return StringUtils.countMatches(text, needle);
	}
}
