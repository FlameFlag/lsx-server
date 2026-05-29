/* ###
 * Pre-analysis Lemonade Tycoon 2 anti-disassembly setup.
 *
 * Run this with ghidra-analyzeHeadless -preScript before normal auto-analysis.
 * It disables decompiler-backed analyzers that eagerly walk unresolved
 * protector control-flow before the project has been cleaned.  The actual
 * anti-disassembly patches are applied later by StripAntiDisassembly.java once
 * Ghidra has decoded real instructions instead of raw byte offsets.
 *
 * This is a project-only patch.  Binaries on disk are not modified.
 */
//@category Lemonade Tycoon 2

import com.lemonadetycoon.ghidra.AnalysisScriptSupport;
import com.google.gson.JsonObject;
import java.io.File;
import java.util.Map;

public class PreRepairAntiDisassembly extends AnalysisScriptSupport {

	private static final String DEFAULT_LOG =
		"decompiled/analysis/pre_repair_antidisasm.jsonl";
	private static final String EVENT_SCHEMA = "lt2.analysis_event.v1";
	private static final String STAGE = "pre_repair_antidisasm";

	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("PreRepairAntiDisassembly: no current program");
			return;
		}

			File log = resolveProjectLogFile("lemonade.preRepair.log",
				"lt2.preRepair.log", DEFAULT_LOG);
			ensureFreshLogFile(log);
		int changed = 0;
		changed += setAnalyzerOption(log, "Decompiler Parameter ID", "false");
		changed += setAnalyzerOption(log, "Decompiler Switch Analysis", "false");

		println("PreRepairAntiDisassembly: configured " + changed +
			" pre-analysis option(s); decoded edge repair will run post-analysis");
	}

	private int setAnalyzerOption(File log, String option, String value) throws Exception {
		Map<String, String> options = getCurrentAnalysisOptionsAndValues(currentProgram);
		if (!options.containsKey(option)) {
			appendLog(log, option, "", "", "missing analyzer option");
			return 0;
		}
		String before = options.get(option);
		if (value.equals(before)) {
			appendLog(log, option, before, value, "already configured");
			return 0;
		}
		try {
			setAnalysisOption(currentProgram, option, value);
			appendLog(log, option, before, value, "updated analyzer option");
			return 1;
		}
		catch (Exception e) {
			appendLog(log, option, before, value, "failed: " + e.getMessage());
			return 0;
		}
	}

	private void appendLog(File file, String option, String before, String after, String note)
			throws Exception {
		JsonObject event = new JsonObject();
		event.addProperty("schema", EVENT_SCHEMA);
		event.addProperty("stage", STAGE);
		event.addProperty("program", currentProgram.getName());
		event.addProperty("executable_md5", currentProgram.getExecutableMD5());
		event.addProperty("action", "set_analysis_option");
		event.addProperty("category", "analysis_option");
		event.addProperty("option", option);
		event.addProperty("old_value", before);
		event.addProperty("new_value", after);
		event.addProperty("decision", noteDecision(note));
		event.addProperty("evidence", note);
		appendJsonLine(file, event);
	}

	private String noteDecision(String note) {
		if (note.startsWith("updated")) {
			return "updated";
		}
		if (note.startsWith("already")) {
			return "already_configured";
		}
		if (note.startsWith("missing")) {
			return "missing";
		}
		if (note.startsWith("failed")) {
			return "failed";
		}
		return "observed";
	}

}
