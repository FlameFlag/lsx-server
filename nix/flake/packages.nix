{ self, ... }:

{
  perSystem =
    { pkgs, ... }:
    let
      shared = import ./lib.nix { inherit pkgs self; };
      inherit (shared)
        goVendorHash
        pythonAnalysisEnv
        version
        webNodeModules
        ;

      buildGoTool = pkgs.callPackage ../packages/go-tool.nix {
        inherit
          goVendorHash
          self
          version
          ;
      };

      lsxServer = pkgs.callPackage ../packages/lsx-server.nix {
        inherit
          goVendorHash
          self
          version
          webNodeModules
          ;
      };
      lsxServerWeb = pkgs.callPackage ../packages/lsx-server-web.nix {
        inherit
          self
          version
          webNodeModules
          ;
      };
      keyvisSearch = pkgs.callPackage ../packages/keyvis-search.nix {
        inherit self version;
      };
      ghidraScripts = pkgs.callPackage ../packages/ghidra-scripts.nix {
        inherit self version;
      };
      lt2rb = buildGoTool {
        pname = "lt2rb";
        subPackage = "./tools/lt2rb";
      };
      lt2findings = buildGoTool {
        pname = "lt2findings";
        subPackage = "./tools/lt2findings";
        cgo = true;
        checkPhase = ''
          runHook preCheck
          go test ./tools/lt2findings
          go run ./tools/lt2findings -check
          runHook postCheck
        '';
      };
      lt2install = buildGoTool {
        pname = "lt2install";
        subPackage = "./tools/lt2install";
      };
      lt2keygen = buildGoTool {
        pname = "lt2keygen";
        subPackage = "./tools/lt2keygen";
      };
      lt2normalize = buildGoTool {
        pname = "lt2normalize";
        subPackage = "./tools/lt2normalize";
      };
      shortv3derive = buildGoTool {
        pname = "shortv3derive";
        subPackage = "./tools/shortv3derive";
      };
    in
    {
      packages = {
        default = lsxServer;
        lsx-server = lsxServer;
        lsx-server-web = lsxServerWeb;
        inherit
          lt2findings
          lt2install
          lt2keygen
          lt2normalize
          lt2rb
          shortv3derive
          ;
        keyvis-search = keyvisSearch;
        ghidra-scripts = ghidraScripts;
        lemonade2-static-unpacking-env = pythonAnalysisEnv;
      };
    };
}
