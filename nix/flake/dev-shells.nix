{ self, ... }:

{
  perSystem =
    { lib, pkgs, ... }:
    let
      inherit (pkgs.stdenv.hostPlatform) isDarwin isx86_64;
      shared = import ./lib.nix { inherit pkgs self; };
      inherit (shared)
        nodejs
        psScriptAnalyzer
        pythonAnalysisEnv
        webNodeModules
        ;
    in
    {
      devShells.default = pkgs.mkShell {
        npmDeps = webNodeModules;
        npmRoot = "go/web";

        packages = [
          pythonAnalysisEnv
          pkgs.bash
          pkgs.bzip2
          pkgs.clang
          pkgs.git
          pkgs.golangci-lint
          pkgs.go_1_26
          pkgs.gradle
          pkgs.gnumake
          pkgs.jdk21_headless
          pkgs.biome
          nodejs
          pkgs.importNpmLock.hooks.linkNodeModulesHook
          pkgs.powershell
          pkgs.shellcheck
          pkgs.shfmt
        ]
        ++ lib.optionals (!(isDarwin && isx86_64)) [
          pkgs.sage
        ];

        shellHook = ''
          export JAVA_HOME="${pkgs.jdk21_headless.home}"
          export PSModulePath="${psScriptAnalyzer}/share/powershell/Modules''${PSModulePath:+:}$PSModulePath"
        '';
      };
    };
}
