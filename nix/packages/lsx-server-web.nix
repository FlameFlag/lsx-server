{
  importNpmLock,
  nodejs_24,
  self,
  stdenvNoCC,
  version,
  webNodeModules,
}:

stdenvNoCC.mkDerivation {
  pname = "lsx-server-web";
  inherit version;

  src = self + "/go";
  npmDeps = webNodeModules;
  npmRoot = "web";

  nativeBuildInputs = [
    nodejs_24
    importNpmLock.hooks.linkNodeModulesHook
  ];

  buildPhase = ''
    runHook preBuild
    npm --prefix="$npmRoot" run generate:openapi
    npm --prefix="$npmRoot" run check
    npm --prefix="$npmRoot" run build
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mkdir -p "$out/share/lsx-server-web"
    cp -R "$npmRoot/dist"/. "$out/share/lsx-server-web/"
    runHook postInstall
  '';
}
