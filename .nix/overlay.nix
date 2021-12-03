self: super: {
	go = super.go.overrideAttrs (old: {
		version = "1.17";
		src = builtins.fetchurl {
			url    = "https://golang.org/dl/go1.17.linux-amd64.tar.gz";
			sha256 = "sha256:0b9p61m7ysiny61k4c0qm3kjsjclsni81b3yrxqkhxmdyp29zy3b";
		};
		doCheck = false;
		patches = [
			# cmd/go/internal/work: concurrent ccompile routines
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/4e07fa9fe4e905d89c725baed404ae43e03eb08e.patch")
			# cmd/cgo: concurrent file generation
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/432db23601eeb941cf2ae3a539a62e6f7c11ed06.patch")
		];
	});
	buildGoModule = super.buildGoModule.override {
		inherit (self) go;
	};
	# gtk4 = super.gtk4.overrideAttrs (old: {
	# 	version = "4.5.0-031aab3";
	# 	src = super.fetchFromGitLab {
	# 		domain = "gitlab.gnome.org";
	# 		owner  = "GNOME";
	# 		repo   = "gtk";
	# 		# commit: Unrealize ATContext on unroot
	# 		rev    = "031aab3ef6633dbea1ead675b0dbdbf562efe5ee";
	# 		sha256 = "0rxc78p4qnwbcwdgkm2ks1nhz04qzyjivcw7iq1ypp5b2bwfvlys";
	# 	};
	# 	buildInputs = old.buildInputs ++ (with super; [ xorg.libXdamage ]);
	# });
	# pango = super.pango.overrideAttrs (old: {
	# 	version = "1.49.4";
	# 	src = super.fetchFromGitLab {
	# 		domain = "gitlab.gnome.org";
	# 		owner  = "GNOME";
	# 		repo   = "pango";
	# 		# v1.49.4
	# 		rev    = "24ca0e22b8038eba7c558eb19f593dfc4892aa55";
	# 		sha256 = "1z8bdy5p1v5vl4kn0rkl80cyw916vxxf7r405jrfkm6zlarc4338";
	# 	};
	# 	buildInputs = old.buildInputs ++ (with super; [ json-glib ]);
	# });
}
