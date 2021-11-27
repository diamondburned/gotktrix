{ pkgs, internalPkgs ? import ./pkgs.nix {} }:

let nix-bundle-src = pkgs.fetchFromGitHub {
		owner  = "matthewbauer";
		repo   = "nix-bundle";
		rev    = "223f4ff";
		sha256 = "0pqpx9vnjk9h24h9qlv4la76lh5ykljch6g487b26r1r2s9zg7kh";
	};

	nix-bundle  = import nix-bundle-src {
		nixpkgs = internalPkgs;
	};

in nix-bundle.nix-bootstrap {
	target = internalPkgs.callPackage ./package.nix {
		src = ./..;
	};
	run = "/bin/gotktrix";
}
