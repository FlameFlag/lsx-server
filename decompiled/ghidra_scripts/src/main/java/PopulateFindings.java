/* ###
 * Lemonade Tycoon 2 reverse-engineering helper script.
 *
 * Run this from Ghidra's Script Manager or as an analyzeHeadless post-script.
 * It annotates focused files extracted by tools/lt2install using
 * decompiled/findings/findings.ini as the source of truth.
 */
//@category Lemonade Tycoon 2

import ghidra.program.model.address.Address;
import ghidra.program.database.sourcemap.SourceFile;
import ghidra.program.model.listing.BookmarkManager;
import ghidra.program.model.listing.BookmarkType;
import ghidra.program.model.listing.Function;
import ghidra.program.model.sourcemap.SourceFileManager;
import ghidra.program.model.sourcemap.SourceMapEntry;
import ghidra.program.model.symbol.SourceType;
import ghidra.program.model.symbol.Symbol;
import ghidra.program.model.symbol.SymbolTable;
import ghidra.program.model.util.PropertyMapManager;
import ghidra.program.model.util.StringPropertyMap;
import ghidra.util.exception.InvalidInputException;
import com.lemonadetycoon.ghidra.AnalysisScriptSupport;
import com.lemonadetycoon.ghidra.findings.CSourceSnippets;
import com.lemonadetycoon.ghidra.findings.Finding;
import com.lemonadetycoon.ghidra.findings.FindingsFile;
import java.io.File;
import java.util.List;
import java.util.Map;

public class PopulateFindings extends AnalysisScriptSupport {

	private static final String CATEGORY = "Lemonade Tycoon 2 Findings";
	private static final String PROPERTY_PREFIX = "lt2.finding.";
	private static final String DEFAULT_FINDINGS_PATH = "decompiled/findings/findings.ini";
	private static final String DEFAULT_C_SOURCE_DIR = "decompiled/src";
	@Override
	public void run() throws Exception {
		if (currentProgram == null) {
			println("No current program. Import/process one extracted Lemonade Tycoon 2 file first.");
			return;
		}

		String name = currentProgram.getName();
		String md5 = currentProgram.getExecutableMD5();
		println("Lemonade Tycoon 2 populate findings: " + name + " md5=" + md5);
		if (!Boolean.getBoolean("lemonade.populate.skipAuto") &&
			!Boolean.getBoolean("lt2.populate.skipAuto")) {
			runAutomaticResolutionPipeline();
		}

		List<Finding> findings = loadFindings();
		Map<String, String> cSnippets = loadCSnippets(findings);
		Map<String, CSourceSnippets.Location> cLocations = loadCLocations(findings);
		int applied = 0;
		for (Finding finding : findings) {
			if (finding.program().equalsIgnoreCase(name)) {
				applyFinding(finding, cSnippets, cLocations);
				applied++;
			}
		}
		if (applied == 0) {
			println("No extracted-file Lemonade Tycoon 2 annotations defined for this program.");
		}
		else {
			println("Applied " + applied + " Lemonade Tycoon 2 finding annotation(s) from the findings file.");
		}
	}

	private void runAutomaticResolutionPipeline() throws Exception {
		runAutoStage("Borland demangler", "BorlandDemangler.java");
		runAutoStage("Windows types", "ApplyWindowsTypes.java");
		runAutoStage("CRT resolver", "ResolveCrtWrappers.java");
		runAutoStage("IAT thunk resolver", "ResolveImportThunks.java");
	}

	private void runAutoStage(String label, String scriptName) throws Exception {
		int before = countDefaultFunctions();
		println("Lemonade Tycoon 2 auto stage: " + label + " starting (FUN_*=" + before + ")");
		runScript(scriptName);
		int after = countDefaultFunctions();
		println("Lemonade Tycoon 2 auto stage: " + label + " finished (FUN_* " +
			before + " -> " + after + ")");
	}

	private void applyFinding(Finding finding, Map<String, String> cSnippets,
			Map<String, CSourceSnippets.Location> cLocations) throws Exception {
		Address address = addr(finding.address());
		String record = finding.xmacroRecord();
		String comment = recordWithCSource(record, finding, cSnippets);

		if ("function".equals(finding.kind())) {
			ensureFunction(address, finding.label());
		}
		label(address, finding.label());
		if ("function".equals(finding.kind())) {
			tagFunction(address, finding);
			plate(address, comment);
		}
		else {
			pre(address, comment);
		}
		bookmark(address, finding, record);
		storeFindingProperties(address, finding, record);
		sourceMap(address, finding, cLocations);
	}

	private String recordWithCSource(String record, Finding finding,
			Map<String, String> cSnippets) {
		if (finding.cSymbol() == null || finding.cSymbol().length() == 0) {
			return record;
		}
		String snippet = cSnippets.get(finding.cSymbol());
		if (snippet == null || snippet.length() == 0) {
			return record;
		}
		return record + "\n\nRecovered C for " + finding.cSymbol() + ":\n" + snippet;
	}

	private List<Finding> loadFindings() throws Exception {
		return FindingsFile.load(resolveFindingsFile());
	}

	private File resolveFindingsFile() {
		String override = System.getProperty("lemonade.findings",
			System.getProperty("lt2.findings", DEFAULT_FINDINGS_PATH));
		File findingsFile = new File(override);
		if (findingsFile.isFile()) {
			return findingsFile;
		}
		return new File(System.getProperty("user.dir"), override);
	}

	private File resolveCSourceDir() {
		String override = System.getProperty("lemonade.csrc",
			System.getProperty("lt2.csrc", DEFAULT_C_SOURCE_DIR));
		File dir = new File(override);
		if (dir.isDirectory()) {
			return dir;
		}
		return new File(System.getProperty("user.dir"), override);
	}

	private Address addr(String text) {
		return toAddr(text);
	}

	private void label(Address address, String name) {
		try {
			SymbolTable symbols = currentProgram.getSymbolTable();
			Symbol existing = getSymbolAt(address);
			if (existing == null || existing.getSource() == SourceType.DEFAULT) {
				symbols.createLabel(address, name, SourceType.USER_DEFINED);
			}
		}
		catch (InvalidInputException e) {
			println("Could not label " + address + " as " + name + ": " + e.getMessage());
		}
	}

	private void ensureFunction(Address address, String name) {
		if (getFunctionAt(address) != null) {
			return;
		}
		try {
			createFunction(address, name);
		}
		catch (Exception e) {
			println("Could not create function " + name + " at " + address + ": " +
				e.getMessage());
		}
	}

	private void plate(Address address, String comment) {
		Function function = getFunctionAt(address);
		if (function != null) {
			setPlateComment(function.getEntryPoint(), comment);
		}
		else {
			setPlateComment(address, comment);
		}
	}

	private void pre(Address address, String comment) {
		setPreComment(address, comment);
	}

	private void tagFunction(Address address, Finding finding) {
		Function function = getFunctionAt(address);
		if (function == null) {
			return;
		}
		for (String tag : findingTags(finding)) {
			try {
				function.addTag(tag);
			}
			catch (Exception e) {
				println("Could not tag " + function.getName() + " with " + tag + ": " +
					e.getMessage());
			}
		}
	}

	private String[] findingTags(Finding finding) {
		String domain = finding.id();
		int dot = domain.indexOf('.');
		if (dot >= 0) {
			domain = domain.substring(0, dot);
		}
		return new String[] {
			"LT2",
			"LT2_KIND_" + safeTagPart(finding.kind()),
			"LT2_PROGRAM_" + safeTagPart(finding.program()),
			"LT2_DOMAIN_" + safeTagPart(domain),
			"LT2_FINDING_" + safeTagPart(finding.id())
		};
	}

	private String safeTagPart(String value) {
		if (value == null || value.length() == 0) {
			return "unknown";
		}
		StringBuilder out = new StringBuilder();
		for (int i = 0; i < value.length(); i++) {
			char ch = value.charAt(i);
			if ((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
				(ch >= '0' && ch <= '9')) {
				out.append(ch);
			}
			else {
				out.append('_');
			}
		}
		return out.toString().replaceAll("_+", "_").replaceAll("^_|_$", "");
	}

	private void bookmark(Address address, Finding finding, String comment) {
		BookmarkManager bookmarks = currentProgram.getBookmarkManager();
		bookmarks.setBookmark(address, BookmarkType.NOTE, CATEGORY + "/" + finding.id(),
			finding.title() + ": " + comment);
	}

	private void storeFindingProperties(Address address, Finding finding, String record) {
		try {
			stringProperty(PROPERTY_PREFIX + "id").add(address, finding.id());
			stringProperty(PROPERTY_PREFIX + "kind").add(address, finding.kind());
			stringProperty(PROPERTY_PREFIX + "label").add(address, finding.label());
			stringProperty(PROPERTY_PREFIX + "title").add(address, finding.title());
			stringProperty(PROPERTY_PREFIX + "source_file").add(address, finding.sourceFile());
			stringProperty(PROPERTY_PREFIX + "c_symbol").add(address, finding.cSymbol());
			stringProperty(PROPERTY_PREFIX + "record").add(address, record);
			stringProperty(PROPERTY_PREFIX + "comment").add(address, finding.comment());
		}
		catch (Exception e) {
			println("Could not store LT2 finding properties at " + address + ": " +
				e.getMessage());
		}
	}

	private StringPropertyMap stringProperty(String name) throws Exception {
		PropertyMapManager properties = currentProgram.getUsrPropertyManager();
		StringPropertyMap map = properties.getStringPropertyMap(name);
		if (map != null) {
			return map;
		}
		return properties.createStringPropertyMap(name);
	}

	private void sourceMap(Address address, Finding finding,
			Map<String, CSourceSnippets.Location> cLocations) {
		if (finding.cSymbol() == null || finding.cSymbol().length() == 0) {
			return;
		}
		CSourceSnippets.Location location = cLocations.get(finding.cSymbol());
		if (location == null) {
			return;
		}
		try {
			SourceFileManager sourceFiles = currentProgram.getSourceFileManager();
			String path = location.path().toString().replace(File.separatorChar, '/');
			SourceFile sourceFile = new SourceFile(path);
			if (!sourceFiles.containsSourceFile(sourceFile)) {
				sourceFiles.addSourceFile(sourceFile);
			}
			for (SourceMapEntry entry : sourceFiles.getSourceMapEntries(address)) {
				if (entry.getSourceFile().equals(sourceFile) &&
					entry.getLineNumber() == location.line() &&
					entry.getLength() == 0) {
					return;
				}
			}
			sourceFiles.addSourceMapEntry(sourceFile, location.line(), address, 0);
		}
		catch (Exception e) {
			println("Could not source-map " + finding.cSymbol() + " at " +
				address + ": " + e.getMessage());
		}
	}

	private Map<String, String> loadCSnippets(List<Finding> findings) throws Exception {
		File root = resolveCSourceDir();
		if (!root.isDirectory()) {
			println("C source directory not found: " + root.getPath());
		}
		return CSourceSnippets.load(root, findings);
	}

	private Map<String, CSourceSnippets.Location> loadCLocations(List<Finding> findings)
			throws Exception {
		File root = resolveCSourceDir();
		if (!root.isDirectory()) {
			return Map.of();
		}
		return CSourceSnippets.locate(root, findings);
	}
}
