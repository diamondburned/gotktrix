{ pkgs }:

let nix-bundle = pkgs.fetchFromGitHub {
		owner  = "matthewbauer";
		repo   = "nix-bundle";
		rev    = "223f4ff";
		sha256 = "0pqpx9vnjk9h24h9qlv4la76lh5ykljch6g487b26r1r2s9zg7kh";
	};
	src = import ./src.nix;

	withPkgs' = {
		overlays = [ (import ./overlay.nix) ];
	};

	withPkgs = import src.nixpkgs  withPkgs';
	muslPkgs = import src.nixpkgs (withPkgs' // {
		localSystem.config = "x86_64-unknown-linux-musl";
	});

	appimage = pkgs.callPackage (nix-bundle + "/appimage.nix") {
		appimagetool = withPkgs.callPackage (nix-bundle + "/appimagetool.nix") {};
	};

	appdir = pkgs.callPackage (nix-bundle + "/appdir.nix") {
		inherit muslPkgs;
	};

in appimage (appdir {
	name = "gotktrix";
	target = pkgs.callPackage ./package.nix {
		# internalPkgs = muslPkgs;
	};
})
