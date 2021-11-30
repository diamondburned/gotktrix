{ pkgs }:

let src = import ../src.nix;
	nativePkgs = import ../pkgs.nix {};

	muslOverlays = self: super: {
		# These dependencies don't have to be native, so we can use the system's.
		git = pkgs.git;
		# go = nativePkgs.go;
		# buildGoModule = nativePkgs.buildGoModule;

		gst_all_1 = {
			gst-plugins-base = super.hello;
			gst-plugins-bad  = super.hello;
		};
		# We don't need these in GTK4.
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
			doCheck = false;
		});
		# We don't want systemd.
		procps = super.procps.override {
			withSystemd = false;
		};
		# We don't need GLib docs.
		glib = super.glib.overrideAttrs (old: {
			mesonFlags = old.mesonFlags ++ [ "-Dgtk_doc=false" ];
			# outputs = [ "bin" "out" "dev" ];
			doCheck = false;
		});
		# nor gobject-introspection.
		gobject-introspection = super.gobject-introspection.overrideAttrs (old: {
			mesonFlags = old.mesonFlags ++ [ "-Dgtk_doc=false" ];
			outputs = [ "out" "dev" ];
			doCheck = false;
		});
		# nor harfbuzz.
		harfbuzz = super.harfbuzz.overrideAttrs (old: {
			outputs = [ "out" "dev" ];
			mesonFlags = old.mesonFlags ++ [ "-Ddocs=disabled" ];
			doCheck = false;
		});
		pango = super.pango.overrideAttrs (old: {
			NIX_CFLAGS_COMPILE = "-w";
			mesonFlags = old.mesonFlags ++ [
				"-Dgtk_doc=false"
				"-Dinstall-tests=false"
				"-Dintrospection=disabled"
			];
			outputs = [ "bin" "out" "dev" ];
			postInstall = "";
			doCheck = false;
		});
		graphene = super.graphene.overrideAttrs (old: {
			mesonFlags = old.mesonFlags ++ [ "-Dgtk_doc=false" ];
			outputs = [ "out" "installedTests" ];
			doCheck = false;
		});
		# gtk-doc = pkgs.hello;
	};

	withPkgs' = {
		crossSystem = {
			config = "x86_64-unknown-linux-musl";
		};
		overlays = [
			(import ../overlay.nix)
			(muslOverlays)
		];
	};

	withPkgs = import src.nixpkgs withPkgs';
	muslPkgs = withPkgs.pkgsMusl;

in muslPkgs.callPackage ../package.nix {
	src = ./../..;
	buildPkgs = muslPkgs;
}
