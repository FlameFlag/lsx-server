/* ###
 * Lemonade Tycoon 2 Borland CRT/STL wrapper resolver.
 *
 * Decompiles default-named functions and matches only high-signal Lemonade Tycoon 2 CRT
 * idioms.  The script writes durable JSONL analysis events consumed by tools/lt2findings.
 */
//@category Lemonade Tycoon 2

import ghidra.app.decompiler.DecompInterface;
import ghidra.app.decompiler.DecompileOptions;
import ghidra.app.decompiler.DecompileResults;
import ghidra.program.model.listing.Function;
import ghidra.program.model.listing.FunctionIterator;
import ghidra.program.model.listing.Instruction;
import ghidra.program.model.listing.InstructionIterator;
import ghidra.program.model.symbol.SourceType;
import com.lemonadetycoon.ghidra.AnalysisScriptSupport;
import com.google.gson.JsonObject;
import java.io.File;
import java.util.Locale;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.apache.commons.lang3.StringUtils;

public class ResolveCrtWrappers extends AnalysisScriptSupport {

	private static final String DEFAULT_LOG = "decompiled/analysis/resolved_crt_wrappers.jsonl";
	private static final String EVENT_SCHEMA = "lt2.analysis_event.v1";
	private static final String STAGE = "crt_resolver";
	private static final Pattern DEFAULT_ARG_WRAPPER =
		Pattern.compile("return\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*\\([^;]*,\\s*(?:0|false|NULL)\\s*\\)\\s*;");

	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("ResolveCrtWrappers: no current program");
			return;
		}
		int before = countDefaultFunctions();
		int considered = 0;
		int renamed = 0;
		File log = resolveProjectLogFile("lemonade.crt.log", "lt2.crt.log", DEFAULT_LOG);
		ensureFreshLogFile(log);

		DecompInterface decompiler = new DecompInterface();
		decompiler.setOptions(new DecompileOptions());
		decompiler.openProgram(currentProgram);

		int transaction = currentProgram.startTransaction("Lemonade Tycoon 2 resolve CRT wrappers");
		try {
			FunctionIterator functions =
				currentProgram.getFunctionManager().getFunctions(true);
			while (functions.hasNext() && !monitor.isCancelled()) {
				Function function = functions.next();
				if (function.getName().startsWith("CRT_")) {
					String oldName = function.getName();
					String badFlow = findBadFlow(function);
					if (badFlow != null) {
						String reviewName = rename(function, "AutoReview_BadFlow");
						if (reviewName != null) {
							setPlateComment(function.getEntryPoint(),
								"Lemonade Tycoon 2 auto-resolution rejected before decompile: unsafe flow\n" +
								"Previous name: " + oldName + "\n" +
								"Evidence: " + badFlow);
							renamed++;
						}
						continue;
					}
					String stableName = nameWithEntryAddress(oldName, function);
					if (!oldName.equals(stableName)) {
						try {
							function.setName(stableName, SourceType.ANALYSIS);
							oldName = stableName;
						}
						catch (Exception e) {
							println("ResolveCrtWrappers: could not stabilize " +
								function.getEntryPoint() + ": " + e.getMessage());
						}
						}
						appendLog(log, function, oldName, function.getName(),
							new Match(function.getName(), 100, "existing resolved CRT name"),
							"confirmed_existing");
						continue;
					}
				if (!function.getName().startsWith("FUN_")) {
					continue;
				}
				considered++;
				Match staticMatch = staticNoisyFunctionMatch(function);
				if (staticMatch != null) {
					String oldName = function.getName();
					String newName = rename(function, staticMatch.name);
					if (newName != null) {
						setPlateComment(function.getEntryPoint(),
							"Lemonade Tycoon 2 auto-resolved without decompilation\n" +
							"Confidence: " + staticMatch.confidence +
							"\nEvidence: " + staticMatch.evidence);
							if (staticMatch.name.startsWith("CRT_")) {
								appendLog(log, function, oldName, newName, staticMatch, "renamed");
							}
						renamed++;
					}
					continue;
				}
				String badFlow = findBadFlow(function);
				if (badFlow != null) {
					String oldName = function.getName();
					String reviewName = rename(function, "AutoReview_BadFlow");
					if (reviewName != null) {
						setPlateComment(function.getEntryPoint(),
							"Lemonade Tycoon 2 auto-resolution rejected before decompile: unsafe flow\n" +
							"Previous name: " + oldName + "\n" +
							"Evidence: " + badFlow);
						renamed++;
					}
					continue;
				}
				DecompileResults results = decompiler.decompileFunction(function, 20, monitor);
				if (results == null || !results.decompileCompleted() ||
					results.getDecompiledFunction() == null) {
					continue;
				}
				String c = results.getDecompiledFunction().getC();
				if (isUnsafeDecompile(c)) {
					String oldName = function.getName();
					String reviewName = rename(function, "AutoReview_BadFlow");
					if (reviewName != null) {
						setPlateComment(function.getEntryPoint(),
							"Lemonade Tycoon 2 auto-resolution rejected: unsafe decompiler flow\n" +
							"Previous name: " + oldName + "\n" +
							"Evidence: overlapping/bad instruction or unmapped opaque branch");
						renamed++;
					}
					continue;
				}
				Match match = classify(c);
				if (match == null || match.confidence < 80) {
					continue;
				}
				String oldName = function.getName();
				String newName = rename(function, match.name);
				if (newName == null) {
					continue;
				}
				setPlateComment(function.getEntryPoint(),
					"Lemonade Tycoon 2 auto-resolved CRT wrapper: " + match.name +
					"\nConfidence: " + match.confidence +
					"\nEvidence: " + match.evidence);
					appendLog(log, function, oldName, newName, match, "renamed");
				renamed++;
			}
		}
		finally {
			decompiler.dispose();
			currentProgram.endTransaction(transaction, true);
		}

		int after = countDefaultFunctions();
		println("ResolveCrtWrappers: considered " + considered + " FUN_* function(s), renamed " +
			renamed + ", FUN_* " + before + " -> " + after);
	}

	private Match classify(String c) {
		String compact = StringUtils.normalizeSpace(c);
		String lower = compact.toLowerCase(Locale.ROOT);
		if (containsAll(lower, "0x1c", "free(") &&
			(lower.contains("+= -1") || lower.contains("-= 1"))) {
			return new Match("CRT_RefCountRelease", 92, "refcount decrement/free idiom");
		}
		if (containsAll(lower, "0x1c", "+= 1") &&
			(lower.contains("*this") || lower.contains("this =") ||
			 lower.contains("param_1"))) {
			return new Match("CRT_RefCountAcquire", 88, "refcount increment/copy idiom");
		}
		if (containsAll(lower, "0xffff", "&", "param_1") &&
			(lower.contains("short") || lower.contains("ushort") ||
			 lower.contains("wchar") || lower.contains("param_1 & 0xffff"))) {
			return new Match("CRT_StringAssign", 84, "wide short/long assign selector");
		}
		if (containsAll(lower, "wchar", "0x1c") &&
			(countOccurrences(lower, "while") >= 2 || countOccurrences(lower, "if") >= 5)) {
			return new Match("CRT_StringCompare", 82, "nested wide string traversal");
		}
		if (containsAll(lower, "(param_2 < param_1)", "param_3 + (int)param_2",
			"param_3 >> 2") &&
			(lower.contains("could not emulate address calculation") ||
			 lower.contains("treating indirect jump as call"))) {
			return new Match("CRT_MemMove", 93, "Borland optimized overlapping copy idiom");
		}
		if (containsAll(lower, "(param_1 & 1)", "return this") &&
			(lower.contains("operator_delete") || lower.contains("free(") ||
			 lower.contains("iat_") || lower.contains("fun_"))) {
			return new Match("MSVC_ScalarDeletingDestructor", 86,
				"scalar deleting destructor flag/free idiom");
		}
		if (containsAll(lower, "cvar1 + -1", "= 0;", "+ 4", "+ 8") &&
			lower.contains("+ 0xc") &&
			(lower.contains("ivar2 + -1") ||
			 lower.contains("*(char *)(ivar2 + -1)"))) {
			return new Match("MFC_CStringClear", 88, "MFC CString refcount release/reset idiom");
		}
		if (containsAll(lower, "exceptionlist", ",0,0,0)", "fun_") &&
			(lower.contains(",1,0,0,0)") || lower.contains(", 1, 0, 0, 0)"))) {
			return new Match("MFC_MessageWrapper", 80, "MFC message-map wrapper");
		}
		if (containsAll(lower, "0x1f", "<<", ">>") &&
			(lower.contains("(1 <<") || lower.contains("- 1"))) {
			return new Match("CRT_BitRotate", 90, "shift/mask rotate idiom");
		}
		if (containsAll(lower, "0x1f", "~(", "|", "& param_1") &&
			(lower.contains("<<") || lower.contains(">>"))) {
			return new Match("CRT_BitSwap", 88, "complemented mask swap idiom");
		}
		Matcher wrapper = DEFAULT_ARG_WRAPPER.matcher(compact);
		if (wrapper.find()) {
			return new Match("CRT_Wrapper_" + safeIdentifier(wrapper.group(1)), 80,
				"default argument wrapper");
		}
		return null;
	}

	private Match staticNoisyFunctionMatch(Function function) {
		String address = function.getEntryPoint().toString();
		if (!currentProgram.getName().equals("lt2_game_code.exe")) {
			if (currentProgram.getName().equals("lemonade2_stream2_unpacked.dll")) {
				if (address.equals("10009732") || address.equals("1000afaf") ||
					address.equals("1000c68c") || address.equals("1000d585") ||
					address.equals("1000fdda") || address.equals("100204d5")) {
					return new Match("AutoReview_BadPcode", 100,
						"known Lemonade Tycoon 2 protector/runtime function whose decompiler probe emits bad pcode");
				}
			}
			return null;
		}
		if (address.equals("0042a5a0") || address.equals("0042a8e0")) {
			return new Match("CRT_MemMove", 93,
				"static Lemonade Tycoon 2 Borland optimized overlapping copy; skipped decompiler read-byte warning");
		}
		if (address.equals("00415fe0") || address.equals("00422b25") ||
			address.equals("00426ebf") || address.equals("00428355") ||
			address.equals("0042ba90")) {
			return new Match("AutoReview_UnreadableFlow", 100,
				"known Lemonade Tycoon 2 function whose decompiler probe reads outside mapped memory");
		}
		return null;
	}

	private boolean isUnsafeDecompile(String c) {
		String lower = c.toLowerCase(Locale.ROOT);
		return lower.contains("bad instruction") ||
			lower.contains("control flow encountered bad instruction") ||
			lower.contains("overlaps instruction") ||
			lower.contains("could not follow disassembly flow into non-existing memory") ||
			lower.contains("unable to resolve constructor") ||
			lower.contains("failed to resolve varnode") ||
			lower.contains("could not recover jumptable");
	}

	private String findBadFlow(Function function) {
		InstructionIterator instructions =
			currentProgram.getListing().getInstructions(function.getBody(), true);
		while (instructions.hasNext()) {
			Instruction instruction = instructions.next();
			String reason = directBadFlowReason(instruction);
			if (reason != null) {
				return "direct flow at " + instruction.getMinAddress() + ": " + reason;
			}
		}
		return null;
	}

	private String rename(Function function, String base) {
		return renameWithAddressFallback(function, nameWithEntryAddress(base, function),
			"ResolveCrtWrappers");
	}

	private void appendLog(File file, Function function, String oldName, String newName,
			Match match, String action) throws Exception {
		JsonObject event = new JsonObject();
		event.addProperty("schema", EVENT_SCHEMA);
		event.addProperty("stage", STAGE);
		event.addProperty("program", currentProgram.getName());
		event.addProperty("executable_md5", currentProgram.getExecutableMD5());
		event.addProperty("address", function.getEntryPoint().toString());
		event.addProperty("action", action);
		event.addProperty("symbol_name", newName);
		if ("renamed".equals(action)) {
			event.addProperty("old_name", oldName);
			event.addProperty("new_name", newName);
		}
		event.addProperty("category", "crt_wrapper");
		event.addProperty("confidence", match.confidence);
		event.addProperty("decision", "accepted");
		event.addProperty("evidence", match.evidence);
		appendJsonLine(file, event);
	}

	private static class Match {
		String name;
		int confidence;
		String evidence;

		Match(String name, int confidence, String evidence) {
			this.name = name;
			this.confidence = confidence;
			this.evidence = evidence;
		}
	}
}
