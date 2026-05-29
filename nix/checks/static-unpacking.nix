{
  bash,
  clang,
  pythonAnalysisEnv,
  self,
  stdenvNoCC,
  version,
}:

stdenvNoCC.mkDerivation {
  pname = "lemonade2-static-unpacking-check";
  inherit version;

  src = self + "/decompiled/analysis/lemonade2_static_unpacking";

  nativeBuildInputs = [
    bash
    clang
    pythonAnalysisEnv
  ];

  buildPhase = ''
    runHook preBuild
    python -m py_compile $(find . -type f -name '*.py' -print | sort)
    PYTHONPATH=. python -m unittest discover -s tests -p 'test_*.py'
    bash -n workflow/capture.sh
    clang -D__INTELLISENSE__ -fsyntax-only -Wall -Wextra instrumentation/api/hook.c
    clang -D__INTELLISENSE__ -fsyntax-only -Wall -Wextra instrumentation/api/launcher.c
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mkdir -p "$out"
    touch "$out/ok"
    runHook postInstall
  '';
}
