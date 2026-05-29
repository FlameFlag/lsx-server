package com.lemonadetycoon.ghidra.findings;

import java.util.Locale;
import org.apache.commons.io.FilenameUtils;
import org.apache.commons.text.StringEscapeUtils;

public class Finding {
	private static final String INSTALLER_PROGRAM =
		"Lemonade Tycoon 2 - New York City.exe";
	private static final String INSTALLER_FINDING_PREFIX = "license.";

	String id;
	String sourceFile;
	String program;
	String address;
	String kind;
	String label;
	String cSymbol;
	String title;
	String comment;

	void set(String key, String value, int lineNo) throws Exception {
		if ("source_file".equals(key)) {
			sourceFile = value;
		}
		else if ("program".equals(key)) {
			program = value;
		}
		else if ("address".equals(key)) {
			address = value;
		}
		else if ("kind".equals(key)) {
			kind = value;
		}
		else if ("label".equals(key)) {
			label = value;
		}
		else if ("title".equals(key)) {
			title = value;
		}
		else if ("comment".equals(key)) {
			comment = value;
		}
		else if ("c_symbol".equals(key)) {
			cSymbol = value;
		}
		else {
			throw new Exception("findings file line " + lineNo +
				" has unknown field " + key);
		}
	}

	void validate(int lineNo) throws Exception {
		if (sourceFile == null || program == null || address == null ||
			kind == null || label == null || title == null || comment == null) {
			throw new Exception("finding " + id + " near line " + lineNo +
				" is missing required fields");
		}
		if (cSymbol == null) {
			cSymbol = "";
		}
		if (!sourceBasename().equals(program)) {
			throw new Exception("finding " + id + " near line " + lineNo +
				" has program " + program + " but source_file basename " +
				sourceBasename());
		}
		if (INSTALLER_PROGRAM.equals(program) &&
			!id.startsWith(INSTALLER_FINDING_PREFIX)) {
			throw new Exception("finding " + id + " near line " + lineNo +
				" targets the setup EXE without the explicit " +
				INSTALLER_FINDING_PREFIX + " prefix");
		}
	}

	public String xmacroRecord() {
		return "LT2_FINDING(" +
			cString(id) + ", " +
			cString(sourceFile) + ", " +
			cString(program) + ", " +
			"0x" + address.toUpperCase(Locale.ROOT) + "U, " +
			cString(kind) + ", " +
			cString(label) + ", " +
			cString(cSymbol) + ", " +
			cString(title) + ", " +
			cString(comment) + ")";
	}

	public String id() {
		return id;
	}

	public String sourceFile() {
		return sourceFile;
	}

	public String program() {
		return program;
	}

	public String address() {
		return address;
	}

	public String kind() {
		return kind;
	}

	public String label() {
		return label;
	}

	public String cSymbol() {
		return cSymbol;
	}

	public String title() {
		return title;
	}

	public String comment() {
		return comment;
	}

	private String sourceBasename() {
		return FilenameUtils.getName(sourceFile);
	}

	private static String cString(String value) {
		return "\"" + StringEscapeUtils.escapeJava(value) + "\"";
	}
}
