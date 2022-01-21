let nixpkgs_21_11 =
	let pkgs = import <nixpkgs> {};
	in  pkgs.fetchFromGitHub {
		owner = "NixOS";
		repo  = "nixpkgs";
		rev   = "8a70a6808c884282161bd77706927caeac0c11e8";
		hash  = "sha256:1dcw9qxda18vnx0pis5xccn3ha9ds82l1r81k40l865am8507sj5";
	};

	pkgs' = override: import nixpkgs_21_11 (override // {
		overlays = [ (import ./overlay.nix) ];
	});

	pkgs = pkgs' {};
	qemuPkgs = system: pkgs' { inherit system; };

	lib = pkgs.lib;
	
	package = pkgs: name: pkgs.callPackage ./package.nix {
		src = ./..;
		suffix = if (name == "") then "" else "-" + name;
		buildPkgs = pkgs;
		wrapGApps = false;
	};

	shellCopy = pkg: name: attr: sh: pkgs.runCommandLocal
		name
		({
			src = pkg.outPath;
			buildInputs = pkg.buildInputs;
		} // attr)
		''
			mkdir -p $out
			cp -rf $src/* $out/
			chmod -R +w $out
			${sh}
		'';

	wrapGApps = pkg: shellCopy pkg (pkg.name + "-nixos") {
		nativeBuildInputs = with pkgs; [
			wrapGAppsHook
		];
	} "";

	withInterpreter = interpreter: pkg: shellCopy pkg
		("${pkg.name}-${lib.last (lib.splitString "/" interpreter)}")
		{
			nativeBuildInputs = with pkgs; [
				patchelf
			];
		}
		''
			patchelf --set-interpreter ${interpreter} $out/bin/*
		'';

	output = name: packages: pkgs.runCommandLocal name {
		# Join the object of name to packages into a line-delimited list of strings.
		src = with lib; foldr
			(a: b: a + "\n" + b) ""
			(mapAttrsToList (name: pkg: "${name} ${pkg.outPath}") packages);
		buildInputs = with pkgs; [ coreutils ];
	} ''
		mkdir -p $out

		IFS=$'\n' readarray pkgs <<< "$src"

		for pkg in "''${pkgs[@]}"; {
			[[ "$pkg" == "" || "$pkg" == $'\n' ]] && continue

			read -r name path <<< "$pkg"
			cp -rf "$path/bin" "$out/$name"
		}
	'';

	basePkgs = {
		native  = package (pkgs) "";
		aarch64 = package (qemuPkgs "aarch64-linux") "aarch64-linux";
	};

	interpreters = {
		x86_64  = "/lib64/ld-linux-x86-64.so.2";
		aarch64 = "/lib64/ld-linux-aarch64.so.1"; # TODO: confirm
	};

in output "gotktrix-cross" {
	native-nixos  = wrapGApps basePkgs.native;
	aarch64-nixos = wrapGApps basePkgs.aarch64;

	native  = withInterpreter interpreters.x86_64  basePkgs.native;
	aarch64 = withInterpreter interpreters.aarch64 basePkgs.aarch64;
}
