{ pkgs }:

let src = import ./src.nix;

	# muslOverlays = self: super: {
	# 	gst_all_1 = {
	# 		gst-plugins-base = super.hello;
	# 		gst-plugins-bad  = super.hello;
	# 	};
	# 	# We don't need these in GTK4.
	# 	gtk4 = (super.gtk4.override {
	# 		cupsSupport = false;
	# 		trackerSupport = false;
	# 	}).overrideAttrs (old: {
	# 		NIX_CFLAGS_COMPILE = "-w";
	# 		mesonFlags = old.mesonFlags ++ [
	# 			"-Dmedia=false"
	# 			"-Dmedia-ffmpeg=disabled"
	# 			"-Dmedia-gstreamer=disabled"
	# 			"-Dprint-cups=disabled"
	# 			"-Dintrospection=disabled"
	# 			"-Dgtk_doc=false"
	# 			"-Ddemos=false"
	# 			"-Dprint=false"
	# 			"-Dbuild-examples=false"
	# 			"-Dinstall-tests=false"
	# 		];
	# 		outputs = [ "out" "dev" ];
	# 		# This takes care of gtk4-update-icon-cache and gtk-launch, as well as other binaries,
	# 		# none of which we care about. We also don't care about DevHelp.
	# 		postInstall = "";
	# 		# We're not building any examples either.
	# 		postFixup = "";
	# 		preBuild = ''
	# 			# We need this since the examples should be doing it, but we're not building any
	# 			# examples.
	# 			mkdir -p $out/share/icons/hicolor
	# 		'';
	# 	});
	# 	libadwaita = super.libadwaita.overrideAttrs (old: {
	# 		NIX_CFLAGS_COMPILE = "-w";
	# 		mesonFlags = [
	# 			"-Dintrospection=disabled"
	# 			"-Dgtk_doc=false"
	# 			"-Dtests=false"
	# 			"-Dexamples=false"
	# 			"-Dvapi=false"
	# 		];
	# 		outputs = [ "out" "dev" ];
	# 		outputBin = "";
	# 		# This is only needed for the docs.
	# 		postInstall = "";
	# 	});
	# 	# We don't want systemd.
	# 	procps = super.procps.override {
	# 		withSystemd = false;
	# 	};
	# 	# We don't need GLib docs.
	# 	glib = super.glib.overrideAttrs (old: {
	# 		mesonFlags = old.mesonFlags ++ [ "-Dgtk_doc=false" ];
	# 		outputs = [ "bin" "out" "dev" ];
	# 		# The existing postInstall really wants devdoc.
	# 		postInstall = ''
	# 			moveToOutput "share/glib-2.0" "$dev"
	# 			substituteInPlace "$dev/bin/gdbus-codegen" --replace "$out" "$dev"
	# 		'';
	# 	});
	# 	# nor gobject-introspection.
	# 	gobject-introspection = super.gobject-introspection.overrideAttrs (old: {
	# 		mesonFlags = old.mesonFlags ++ [ "-Dgtk_doc=false" ];
	# 		outputs = [ "out" "dev" ];
	# 	});
	# 	# nor harfbuzz.
	# 	harfbuzz = super.harfbuzz.overrideAttrs (old: {
	# 		outputs = [ "out" "dev" ];
	# 		mesonFlags = old.mesonFlags ++ [ "-Ddocs=disabled" ];
	# 	});
	# 	gdk-pixbuf = super.gdk-pixbuf.overrideAttrs (old: {
	# 		# Don't include libtiff, because that doesn't build for some reason.
	# 		propagatedBuildInputs = [ self.glib ] ++ (with super; [ libjpeg libpng ]);
	# 		mesonFlags = old.mesonFlags ++ [
	# 			"-Dtiff=disabled"
	# 			"-Dgtk_doc=false"
	# 			"-Dintrospection=disabled"
	# 		];
	# 		outputs = [ "out" "dev" ];
	# 		doCheck = false;
	# 	});
	# 	pango = super.pango.overrideAttrs (old: {
	# 		NIX_CFLAGS_COMPILE = "-w";
	# 		mesonFlags = old.mesonFlags ++ [
	# 			"-Dgtk_doc=false"
	# 			"-Dinstall-tests=false"
	# 			"-Dintrospection=disabled"
	# 		];
	# 		outputs = [ "bin" "out" "dev" ];
	# 		postInstall = "";
	# 	});
	# 	graphene = super.graphene.overrideAttrs (old: {
	# 		mesonFlags = old.mesonFlags ++ [ "-Dgtk_doc=false" ];
	# 		outputs = [ "out" "installedTests" ];
	# 	});
	# 	gtk-doc = pkgs.hello;
	# };

	muslOverlays = self: super: {
	 	gst_all_1 = {
	 		gst-plugins-base = super.hello;
	 		gst-plugins-bad  = super.hello;
	 	};
	 	gtk4 = (super.gtk4.override {
	 		cupsSupport = false;
	 		trackerSupport = false;
	 	}).overrideAttrs (old: {
	 		NIX_CFLAGS_COMPILE = "-w";
	 		mesonFlags = old.mesonFlags ++ [
	 			"-Dmedia=false"
	 			"-Dmedia-ffmpeg=disabled"
	 			"-Dmedia-gstreamer=disabled"
	 			"-Dprint-cups=disabled"
	 			"-Dintrospection=disabled"
	 			"-Dgtk_doc=false"
	 			"-Ddemos=false"
	 			"-Dprint=false"
	 			"-Dbuild-examples=false"
	 			"-Dinstall-tests=false"
	 		];
	 		outputs = [ "out" "dev" ];
	 		# This takes care of gtk4-update-icon-cache and gtk-launch, as well as other binaries,
	 		# none of which we care about. We also don't care about DevHelp.
	 		postInstall = "";
	 		# We're not building any examples either.
	 		postFixup = "";
	 		preBuild = ''
	 			# We need this since the examples should be doing it, but we're not building any
	 			# examples.
	 			mkdir -p $out/share/icons/hicolor
	 		'';
	 	});
		gdk-pixbuf = super.gdk-pixbuf.overrideAttrs (old: {
			# Don't include libtiff, because that doesn't build for some reason.
			propagatedBuildInputs = [ self.glib ] ++ (with super; [ libjpeg libpng ]);
			mesonFlags = old.mesonFlags ++ [
				"-Dtiff=disabled"
				"-Dgtk_doc=false"
				"-Dintrospection=disabled"
			];
			outputs = [ "out" "dev" ];
			doCheck = false;
		});
		go = super.go.overrideAttrs (old: {
			CGO_ENABLED = "0";
			# GOFLAGS = "-ldflags=all=-extldflags=-static";
			# GOGCCFLAGS = "-fno-PIC -static";
			# CGO_LDFLAGS = "static";

			postConfigure = old.postConfigure + ''
				export CC=${super.stdenv.cc}/bin/x86_64-unknown-linux-musl-cc
				export CXX=${super.stdenv.cc}/bin/x86_64-unknown-linux-musl-cxx
			'';
			preInstall = "";
		});
	};

	withPkgs' = {
		config = {
			doCheckByDefault = false;
		};
		doCheckByDefault = false;
		overlays = [
			(import ./overlay.nix)
			(muslOverlays)
		];
	};

	withPkgs = import src.nixpkgs withPkgs';
	staticPkgs = withPkgs.pkgsStatic;

# in staticPkgs.go
in staticPkgs.callPackage ./package.nix {
	src = ./..;
	internalPkgs = staticPkgs;
}
