let pkgs = import <nixpkgs> {};

in import ./package.nix {
	inherit pkgs;
	lib = pkgs.lib;
}
