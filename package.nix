{
  buildGoModule,
  git,
  lib,
  makeWrapper,
}:
buildGoModule {
  pname = "reviewr";
  version = "0.1.0";

  src = lib.cleanSource ./.;
  vendorHash = "sha256-tumyQI4jw4lF8tieBrgibezQJjEvQlsQ4pH8jMOhzKM=";

  subPackages = ["cmd/reviewr"];
  nativeBuildInputs = [makeWrapper];
  doCheck = false;

  postInstall = ''
    wrapProgram $out/bin/reviewr --prefix PATH : ${lib.makeBinPath [git]}
  '';
  ldflags = [
    "-s"
    "-w"
  ];

  meta = {
    description = "Terminal application for reviewing repository changes";
    homepage = "https://github.com/josephembrey/reviewr";
    license = lib.licenses.mit;
    mainProgram = "reviewr";
  };
}
