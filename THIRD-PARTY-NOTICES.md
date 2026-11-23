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

## Transcodio text-to-speech normalization rules

DaisText adapts pronunciation rules from Transcodio, which is distributed under
the MIT License.

Copyright (c) 2025 Dorian González

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
