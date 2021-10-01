self: super: {
#	libadwaita = super.libadwaita.overrideAttrs (old: {
#		version = "1.0.0-alpha.2";
#
#		src = super.fetchFromGitLab {
#			domain = "gitlab.gnome.org";
#			owner  = "GNOME";
#			repo   = "libadwaita";
#			rev    = "f5932ab4250c8e709958c6e75a1a4941a5f0f386";
#			hash   = "sha256:1yvjdzs5ipmr4gi0l4k6dkqhl9b090kpjc3ll8bv1a6i7yfaf53s";
#		};
#
#		doCheck = false;
#	});
	libadwaita = super.libadwaita.overrideAttrs (old: {
		version = "1.0.0-alpha.3";

		src = super.fetchFromGitLab {
			domain = "gitlab.gnome.org";
			owner  = "GNOME";
			repo   = "libadwaita";
			rev    = "40c19ab2591763a482ebc79c82f1da32eea3bab6";
			hash   = "sha256:1bfxsq6sm0xpp5z4q2h2qvwkkbga2ryr0ny1wvp6ccarr2dzch70";
		};

		doCheck = false;
	});
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
}
