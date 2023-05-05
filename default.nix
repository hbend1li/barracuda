{ buildGoModule, fetchFromGitLab }:
let
  pname = "reaction";
  version = "v0.1";
in buildGoModule {
  inherit pname version;

  src = ./.;
  # src = fetchFromGitLab {
  #   domain = "framagit.org";
  #   owner = "ppom";
  #   repo = pname;
  #   rev = version;
  #   sha256 = "sha256-45ytTNZIbTIUOPBgAdD7o9hyWlJo//izUhGe53PcwNA=";
  # };

  vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";
}
