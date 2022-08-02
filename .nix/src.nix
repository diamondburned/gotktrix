let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "4f498cd56a726dc2ecb19af471cb43bb759708bb";
		hash  = "sha256:0009jbdj2y2vqi522a3r64xf4drp44ghbidf32j6bslswqf3wy4m";
	};
	nixpkgs = systemPkgs.fetchFromGitHub {
		owner = "NixOS";
		repo  = "nixpkgs";
		rev   = "147b03fa8ebf9d5d5f6784f87dc61f0e7beee911"; # release-21.11
		hash  = "sha256:027nvr5q0314dkb35yzh1a03lza06m8x7kv87cifri7jw7dqfd9s";
	};
}
