let pkgsSrc = import ./src.nix;
	pkgs    = import pkgsSrc.nixpkgs {
		overlays = [ (import ./overlay.nix) ];
	};
	
	package = pkgs: pkgs.callPackage ./package.nix {
		src = ./..;
		buildPkgs = pkgs;
	};

	cross = pkgs.pkgsCross;

# Do something with this:
# patchelf --set-interpreter $(ldd gotktrix | grep ld-linux | sed -n 's/.*=> \(.*\) (.*/\1/p') ./gotktripatchelf --set-interpreter $(ldd gotktrix | grep ld-linux | sed -n 's/.*=> \(.*\) (.*/\1/p') ./gotktrixx
# patchelf --set-interpreter /lib64/ld-linux-x86-64.so.2 ./gotktrix

in {
	native         = package pkgs;
	native-musl    = package cross.musl64;
	aarch64-linux  = package cross.raspberryPi;
	aarch64-darwin = package cross.aarch64-darwin;
}
