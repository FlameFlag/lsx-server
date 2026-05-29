{ pkgs, self }:

let
  nodejs = pkgs.nodejs_24;
in
{
  version = self.shortRev or self.dirtyShortRev or "dev";
  goVendorHash = "sha256-5c6kruWTaWB4U8D+Z4ZUDlDEbPJyBrPOVLVUo/zkqxE=";

  inherit nodejs;

  webNodeModules = pkgs.importNpmLock.buildNodeModules {
    npmRoot = ../../go/web;
    inherit nodejs;
  };

  pythonAnalysisEnv = pkgs.callPackage ../packages/lemonade2-static-unpacking-env.nix {
  };
  psScriptAnalyzer = pkgs.callPackage ../packages/psscriptanalyzer.nix { };
}
