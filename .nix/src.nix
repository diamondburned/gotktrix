let systemPkgs = import <nixpkgs> {};

in {
	gotk4 = systemPkgs.fetchFromGitHub {
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
	nixpkgs = systemPkgs.fetchFromGitHub {
		owner  = "NixOS";
		repo   = "nixpkgs";
		rev    = "8ecc61c91a5";
		sha256 = "sha256:0vhajylsmipjkm5v44n2h0pglcmpvk4mkyvxp7qfvkjdxw21dyml";
	};
}
