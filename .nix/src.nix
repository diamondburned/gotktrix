let systemPkgs = import <nixpkgs> {};

in {
	# gotk4-nix = ../../gotk4-nix;
	gotk4-nix = systemPkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "d2bd6577f1867cb740b281baa48a895aed494967";
		hash  = "sha256:02b2h6a6dip2lsw07jm6ch3775gcms6h7hjfll448f7d99ln1b7m";
	};
	nixpkgs = systemPkgs.fetchFromGitHub {
		owner = "NixOS";
		repo  = "nixpkgs";
		rev   = "147b03fa8ebf9d5d5f6784f87dc61f0e7beee911"; # release-21.11
		hash  = "sha256:027nvr5q0314dkb35yzh1a03lza06m8x7kv87cifri7jw7dqfd9s";
	};
}
