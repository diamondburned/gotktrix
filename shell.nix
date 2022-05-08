{  }:

let flags = {
	# # Uncomment for debugging.
	# hardeningDisable = [ "all" ];
	# NIX_CFLAGS_COMPILE = "-O0 ${die}";
	# CGO_CFLAGS   = "-g2 -O0 ${die}";
	# CGO_CXXFLAGS = "-g2 -O0 ${die}";
	# CGO_FFLAGS   = "-g2 -O0 ${die}";
	# CGO_LDFLAGS  = "-g2 -O0 ${die}";
};

in import ./.nix {
	# TODO: debug = true; for -O0 GTK building
	action = "shell";
}
