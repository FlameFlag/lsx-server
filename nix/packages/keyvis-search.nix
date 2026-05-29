{
  gnumake,
  self,
  stdenv,
  version,
}:

stdenv.mkDerivation {
  pname = "keyvis-search";
  inherit version;

  src = self + "/go/tools/keyvis_search";
  nativeBuildInputs = [ gnumake ];
  enableParallelBuilding = true;

  doCheck = true;
  checkPhase = ''
    runHook preCheck
    ./keyvis-search --version
    ./keyvis-search --help >/dev/null
    runHook postCheck
  '';

  installPhase = ''
    runHook preInstall
    install -Dm755 keyvis-search "$out/bin/keyvis-search"
    runHook postInstall
  '';

  meta.mainProgram = "keyvis-search";
}
