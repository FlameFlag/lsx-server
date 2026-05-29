{
  buildGo126Module,
  goVendorHash,
  self,
  version,
}:

{
  pname,
  subPackage,
  cgo ? false,
  tags ? [ ],
  checkPhase ? ''
    runHook preCheck
    go test ${subPackage}/...
    runHook postCheck
  '',
}:

buildGo126Module {
  inherit
    pname
    tags
    version
    checkPhase
    ;
  src = self;
  modRoot = "go";
  vendorHash = goVendorHash;
  proxyVendor = true;
  env.CGO_ENABLED = if cgo then 1 else 0;
  subPackages = [ subPackage ];
  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];
  meta.mainProgram = pname;
}
