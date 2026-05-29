{
  buildGo126Module,
  goVendorHash,
  importNpmLock,
  lib,
  nodejs_24,
  self,
  version,
  webNodeModules,
}:

buildGo126Module {
  pname = "lsx-server";
  inherit version;
  src = self;
  modRoot = "go";
  vendorHash = goVendorHash;
  proxyVendor = true;
  npmDeps = webNodeModules;
  npmRoot = "go/web";

  nativeBuildInputs = [
    nodejs_24
    importNpmLock.hooks.linkNodeModulesHook
  ];

  overrideModAttrs = oldAttrs: {
    nativeBuildInputs = lib.remove importNpmLock.hooks.linkNodeModulesHook (
      oldAttrs.nativeBuildInputs or [ ]
    );
    preBuild = null;
    preCheck = null;
  };

  env.CGO_ENABLED = 0;
  subPackages = [ "." ];
  tags = [ "webdist" ];
  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  preBuild = ''
    npm --prefix=web run generate:openapi
    npm --prefix=web run build
  '';

  preConfigure = ''
    unset npm_config_workspace npm_config_workspaces NPM_CONFIG_WORKSPACE NPM_CONFIG_WORKSPACES
  '';

  preCheck = ''
    npm --prefix=web run check
    CGO_ENABLED=1 go run ./tools/lt2findings -check
  '';

  checkPhase = ''
    runHook preCheck
    CGO_ENABLED=1 go test -tags webdist ./...
    runHook postCheck
  '';

  postInstall = ''
    if [ -x "$out/bin/lsx_server_go" ] && [ ! -e "$out/bin/lsx-server" ]; then
      mv "$out/bin/lsx_server_go" "$out/bin/lsx-server"
    fi
  '';

  meta.mainProgram = "lsx-server";
}
