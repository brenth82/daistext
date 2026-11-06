# Third-party components

DaisText uses Walk and Win (BSD-style licenses), govaluate (MIT), and Go x/net,
x/sys, and x/text (BSD-style licenses). Their source and license texts are in
the Go module cache during builds. The build copies these licenses into the
installer's tools/licenses directory.

Packaged installers include Typst 0.15.1 (Apache-2.0); its upstream license and
notice files accompany the executable under tools. Source:
https://github.com/typst/typst/tree/v0.15.1

Inno Setup is a build tool, not an installed application dependency. Its license
is available at https://jrsoftware.org/isinfo.php and in .tools/inno after staging.

The optional Liblouis component contains the unmodified upstream Windows build
of Liblouis 3.39.0. DaisText invokes the GPL-3.0-or-later lou_translate executable
as a separate process; the library uses LGPL-2.1-or-later. Each translation table
retains its upstream license notices. Copies of COPYING, COPYING.LESSER and the
upstream build instructions accompany this component under tools/liblouis/licenses.
The complete corresponding source is supplied alongside the installed binaries
at tools/liblouis/source/liblouis-3.39.0.zip, including the Windows build scripts.
Users can configure and substitute their own separately installed translator.
Upstream release: https://github.com/liblouis/liblouis/releases/tag/v3.39.0
