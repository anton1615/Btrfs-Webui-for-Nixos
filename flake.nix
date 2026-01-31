{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in
      {
        packages.default = pkgs.buildGoModule {
          pname = "btrfs-webui-nix";
          version = "0.3.0";
          src = ./.;
          # 這裡設為空，讓它報錯告訴我正確 Hash
          vendorHash = "sha256-aobM88KHWcJW/yjnDqNOlbbOQvfRc2vBfffq/N/Vlo8=";
          buildInputs = [ pkgs.linux-pam ];
        };
      });
}
