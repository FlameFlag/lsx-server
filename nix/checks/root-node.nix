{
  biome,
  self,
  stdenvNoCC,
  version,
}:

stdenvNoCC.mkDerivation {
  pname = "lsx-server-root-node-check";
  inherit version;

  src = self;
  nativeBuildInputs = [ biome ];

  buildPhase = ''
    runHook preBuild
    biome ci
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mkdir -p "$out"
    touch "$out/ok"
    runHook postInstall
  '';
}
