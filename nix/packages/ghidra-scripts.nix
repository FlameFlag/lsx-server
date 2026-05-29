{
  ghidra,
  jdk21_headless,
  self,
  stdenvNoCC,
  version,
}:

stdenvNoCC.mkDerivation {
  pname = "lsx-ghidra-scripts";
  inherit version;

  src = self + "/decompiled/ghidra_scripts";
  nativeBuildInputs = [ jdk21_headless ];

  buildPhase = ''
    runHook preBuild
    export GHIDRA_INSTALL_DIR="${ghidra}/lib/ghidra"
    mkdir -p build/classes
    CLASSPATH="$(find "$GHIDRA_INSTALL_DIR/Ghidra/Framework" \
      "$GHIDRA_INSTALL_DIR/Ghidra/Features" \
      "$GHIDRA_INSTALL_DIR/Ghidra/Debug" \
      "$GHIDRA_INSTALL_DIR/Ghidra/Processors" \
      "$GHIDRA_INSTALL_DIR/support" \
      -type f -name '*.jar' -print | paste -sd ':' -)"
    javac -encoding UTF-8 --release 21 -proc:none \
      -Xlint:deprecation -Xlint:unchecked \
      -cp "$CLASSPATH" \
      -d build/classes \
      $(find src/main/java -type f -name '*.java' -print | sort)
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mkdir -p "$out/share/lsx-ghidra-scripts"
    cp -R src/main/java "$out/share/lsx-ghidra-scripts/src"
    cp -R build/classes "$out/share/lsx-ghidra-scripts/classes"
    runHook postInstall
  '';
}
