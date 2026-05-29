package com.lemonadetycoon.ghidra.findings;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Stream;
import org.apache.commons.lang3.StringUtils;

public final class CSourceSnippets {
	private static final Set<String> IGNORED_DIRS =
		Set.of("generated", "local", "ghidra_projects");

	public record Location(Path path, int line) {
	}

	private CSourceSnippets() {
	}

	public static Map<String, String> load(File root, List<Finding> findings) throws Exception {
		Map<String, String> snippets = new HashMap<String, String>();
		if (!root.isDirectory()) {
			return snippets;
		}

		List<Path> files = collectCFiles(root.toPath());
		for (Finding finding : findings) {
			if (StringUtils.isBlank(finding.cSymbol()) ||
				snippets.containsKey(finding.cSymbol())) {
				continue;
			}
			String snippet = findSnippet(files, finding.cSymbol());
			if (snippet.length() > 0) {
				snippets.put(finding.cSymbol(), snippet);
			}
		}
		return snippets;
	}

	public static Map<String, Location> locate(File root, List<Finding> findings) throws Exception {
		Map<String, Location> locations = new HashMap<String, Location>();
		if (!root.isDirectory()) {
			return locations;
		}

		List<Path> files = collectCFiles(root.toPath());
		for (Finding finding : findings) {
			if (StringUtils.isBlank(finding.cSymbol()) ||
				locations.containsKey(finding.cSymbol())) {
				continue;
			}
			Location location = findLocation(files, finding.cSymbol());
			if (location != null) {
				locations.put(finding.cSymbol(), location);
			}
		}
		return locations;
	}

	private static List<Path> collectCFiles(Path root) throws Exception {
		try (Stream<Path> paths = Files.walk(root)) {
			return paths
				.filter(Files::isRegularFile)
				.filter(CSourceSnippets::isCSource)
				.filter(path -> !hasIgnoredDirectory(root, path))
				.sorted()
				.toList();
		}
	}

	private static boolean isCSource(Path path) {
		String name = path.getFileName().toString();
		return name.endsWith(".c") || name.endsWith(".h");
	}

	private static boolean hasIgnoredDirectory(Path root, Path path) {
		Path relative = root.relativize(path);
		for (Path part : relative) {
			if (IGNORED_DIRS.contains(part.toString())) {
				return true;
			}
		}
		return false;
	}

	private static String findSnippet(List<Path> files, String symbol) throws Exception {
		for (Path file : files) {
			String source = Files.readString(file, StandardCharsets.UTF_8);
			String snippet = extractFunction(source, symbol);
			if (snippet.length() == 0) {
				snippet = extractGlobal(source, symbol);
			}
			if (snippet.length() > 0) {
				return "/* " + file + " */\n" + snippet;
			}
		}
		return "";
	}

	private static Location findLocation(List<Path> files, String symbol) throws Exception {
		for (Path file : files) {
			String source = Files.readString(file, StandardCharsets.UTF_8);
			int symbolAt = findFunctionSymbol(source, symbol);
			if (symbolAt < 0) {
				symbolAt = findGlobalSymbol(source, symbol);
			}
			if (symbolAt >= 0) {
				return new Location(file.toAbsolutePath().normalize(), lineNumber(source, symbolAt));
			}
		}
		return null;
	}

	private static String extractFunction(String source, String symbol) {
		int symbolAt = findFunctionSymbol(source, symbol);
		if (symbolAt < 0) {
			return "";
		}
		int paren = skipWhitespace(source, symbolAt + symbol.length());
		int closeParen = findMatching(source, paren, '(', ')');
		int brace = skipWhitespace(source, closeParen + 1);
		int bodyEnd = findMatching(source, brace, '{', '}');
		if (bodyEnd < 0) {
			return "";
		}
		int start = includeLeadingComment(source, lineStart(source, symbolAt));
		return source.substring(start, bodyEnd + 1).trim();
	}

	private static int findFunctionSymbol(String source, String symbol) {
		int pos = 0;
		while (pos < source.length()) {
			int symbolAt = source.indexOf(symbol, pos);
			if (symbolAt < 0) {
				return -1;
			}
			pos = symbolAt + symbol.length();
			if (!isWordBoundary(source, symbolAt - 1) ||
				!isWordBoundary(source, symbolAt + symbol.length())) {
				continue;
			}
			int paren = skipWhitespace(source, symbolAt + symbol.length());
			if (paren >= source.length() || source.charAt(paren) != '(') {
				continue;
			}
			int closeParen = findMatching(source, paren, '(', ')');
			if (closeParen < 0) {
				continue;
			}
			int brace = skipWhitespace(source, closeParen + 1);
			if (brace >= source.length() || source.charAt(brace) != '{') {
				continue;
			}
			int bodyEnd = findMatching(source, brace, '{', '}');
			if (bodyEnd < 0) {
				continue;
			}
			return symbolAt;
		}
		return -1;
	}

	private static String extractGlobal(String source, String symbol) {
		int symbolAt = findGlobalSymbol(source, symbol);
		if (symbolAt < 0) {
			return "";
		}
		int start = includeLeadingComment(source, lineStart(source, symbolAt));
		int end = source.indexOf('\n', symbolAt);
		if (end < 0) {
			end = source.length();
		}
		return source.substring(start, end).trim();
	}

	private static int findGlobalSymbol(String source, String symbol) {
		int pos = 0;
		while (pos < source.length()) {
			int symbolAt = source.indexOf(symbol, pos);
			if (symbolAt < 0) {
				return -1;
			}
			pos = symbolAt + symbol.length();
			if (!isWordBoundary(source, symbolAt - 1) ||
				!isWordBoundary(source, symbolAt + symbol.length())) {
				continue;
			}
			return symbolAt;
		}
		return -1;
	}

	private static int lineNumber(String source, int offset) {
		int line = 1;
		for (int i = 0; i < offset && i < source.length(); i++) {
			if (source.charAt(i) == '\n') {
				line++;
			}
		}
		return line;
	}

	private static int includeLeadingComment(String source, int start) {
		int cursor = start;
		while (cursor > 0) {
			int before = cursor - 1;
			while (before >= 0 && Character.isWhitespace(source.charAt(before))) {
				before--;
			}
			if (before < 1 || source.charAt(before) != '/' ||
				source.charAt(before - 1) != '*') {
				break;
			}
			int commentStart = source.lastIndexOf("/*", before - 1);
			if (commentStart < 0) {
				break;
			}
			cursor = lineStart(source, commentStart);
		}
		return cursor;
	}

	private static int lineStart(String source, int offset) {
		int start = source.lastIndexOf('\n', offset);
		return start < 0 ? 0 : start + 1;
	}

	private static int skipWhitespace(String source, int offset) {
		while (offset < source.length() && Character.isWhitespace(source.charAt(offset))) {
			offset++;
		}
		return offset;
	}

	private static boolean isWordBoundary(String source, int offset) {
		if (offset < 0 || offset >= source.length()) {
			return true;
		}
		char ch = source.charAt(offset);
		return !(Character.isLetterOrDigit(ch) || ch == '_');
	}

	private static int findMatching(String source, int openAt, char open, char close) {
		int depth = 0;
		boolean inString = false;
		boolean inChar = false;
		boolean inLineComment = false;
		boolean inBlockComment = false;
		for (int i = openAt; i < source.length(); i++) {
			char ch = source.charAt(i);
			char next = i + 1 < source.length() ? source.charAt(i + 1) : '\0';
			if (inLineComment) {
				if (ch == '\n') {
					inLineComment = false;
				}
				continue;
			}
			if (inBlockComment) {
				if (ch == '*' && next == '/') {
					inBlockComment = false;
					i++;
				}
				continue;
			}
			if (inString) {
				if (ch == '\\') {
					i++;
				}
				else if (ch == '"') {
					inString = false;
				}
				continue;
			}
			if (inChar) {
				if (ch == '\\') {
					i++;
				}
				else if (ch == '\'') {
					inChar = false;
				}
				continue;
			}
			if (ch == '/' && next == '/') {
				inLineComment = true;
				i++;
				continue;
			}
			if (ch == '/' && next == '*') {
				inBlockComment = true;
				i++;
				continue;
			}
			if (ch == '"') {
				inString = true;
				continue;
			}
			if (ch == '\'') {
				inChar = true;
				continue;
			}
			if (ch == open) {
				depth++;
			}
			else if (ch == close) {
				depth--;
				if (depth == 0) {
					return i;
				}
			}
		}
		return -1;
	}
}
