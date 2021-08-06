{ systemPkgs ? import <nixpkgs> {} }:

let gotk4 = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4";
		rev   = "4f507c20f8b07f4a87f0152fbefdc9a380042b83";
		hash  = "sha256:0zijivbyjfbb2vda05vpvq268i7vx9bhzlbzzsa4zfzzr9427w66";
	};

	gotk4-adw = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-adwaita";
		rev   = "01f60b73109a41d6b28e09dce61c45486bdc401b";
		hash  = "sha256:1l57ygzg5az0pikn0skj0bwggbvfj21d36glkwpkyp7csxi8hzhr";
	};

	overlay = self: super: {
		libadwaita = super.libadwaita.overrideAttrs (old: {
			version = "1.0.0-alpha.2";
	
			src = super.fetchFromGitLab {
				domain = "gitlab.gnome.org";
				owner  = "GNOME";
				repo   = "libadwaita";
				rev    = "f5932ab4250c8e709958c6e75a1a4941a5f0f386";
				hash   = "sha256:1yvjdzs5ipmr4gi0l4k6dkqhl9b090kpjc3ll8bv1a6i7yfaf53s";
			};

			doCheck = false;
		});
		go = super.go.overrideAttrs (old: {
			version = "1.17rc2";
			src = builtins.fetchurl {
				url    = "https://golang.org/dl/go1.17rc2.linux-amd64.tar.gz";
				sha256 = "sha256:015dg39aj0s6ka5hkqgr9rjmfwz9jzzxgd3cdhfsbln7qznkb0ij";
			};
			doCheck = false;
			patches = [
				# cmd/go/internal/work: concurrent ccompile routines
				(builtins.fetchurl "https://github.com/diamondburned/go/commit/4e07fa9fe4e905d89c725baed404ae43e03eb08e.patch")
				# cmd/cgo: concurrent file generation
				(builtins.fetchurl "https://github.com/diamondburned/go/commit/432db23601eeb941cf2ae3a539a62e6f7c11ed06.patch")
			];
		});
	};

	pkgs = import "${gotk4}/.nix/pkgs.nix" {
		src = systemPkgs.fetchFromGitHub {
			owner  = "NixOS";
			repo   = "nixpkgs";
			rev    = "8ecc61c91a5";
			sha256 = "sha256:0vhajylsmipjkm5v44n2h0pglcmpvk4mkyvxp7qfvkjdxw21dyml";
		};
		overlays = [ (overlay) ];
	};

	shell = import "${gotk4}/.nix/shell.nix" {
		inherit pkgs;
	};

	# minitime is a mini-output time wrapper.
	minitime = pkgs.writeShellScriptBin
		"minitime"
		"command time --format $'🕒 -> %es\\n' \"$@\"";


in shell.overrideAttrs (old: {
	buildInputs = old.buildInputs ++ (with pkgs; [
		gtk4.debug
		glib.debug

		libadwaita
	]);

	nativeBuildInputs = old.nativeBuildInputs ++ [
		minitime
	];

	# NIX_DEBUG_INFO_DIRS = ''${pkgs.gtk4.debug}/lib/debug:${pkgs.glib.debug}/lib/debug'';

	CGO_ENABLED  = "1";
	CGO_CFLAGS   = "-g2 -O2";
	CGO_CXXFLAGS = "-g2 -O2";
	CGO_FFLAGS   = "-g2 -O2";
	CGO_LDFLAGS  = "-g2 -O2";
})
