self: super:

with builtins;

let useOverlay = pathExists "/home/diamond/Scripts/gtk";

in
if (!useOverlay) then ({}) else ({
	gtk4 = super.gtk4.overrideAttrs (old: {
		src = filterSource (path: _: baseNameOf path != ".git") /home/diamond/Scripts/gtk;
		version = "4.7.1";
		outputs = [ "out" "dev" ];
		mesonFlags = [
			"-Dgtk_doc=false"
			"-Dbuild-tests=false"
			"-Dtracker=enabled"
			"-Dbroadway-backend=false"
			"-Dvulkan=disabled"
			"-Dprint-cups=disabled"
			"-Dmedia-gstreamer=disabled"
			"-Dx11-backend=true"
		];
	});

	libadwaita = super.libadwaita.overrideAttrs (old: {
		doCheck = false;
	});
})
