# oksvg
oksvg is a renderer for a partial implementation of the SVG2.0 specification in golang.

Although some SVG elements will not be handled by oksvg, it is good enough to faithfully produce thousands, but not all, SVG icons available both for free and commercially. A list of valid and invalid elements is in the doc folder.

oksvg uses the [rasterx](https://github.com/srwiley/rasterx) adaptation of the golang freetype raster package which implements full SVG2.0 path functions, including the newer 'arc' join-mode.

![arcs and caps](doc/TestShapes.png)

### Extra non-standard features.

In addition to 'arc' as a valid join mode value, oksvg also allows 'arc-clip' which is the arc analog of miter-clip. There are also extra capping and gap functions and different cap functions may be specified for leading and trailing end points.

#### Examples

Rasterizations of a few third party open source SVG icons to PNG images by oksvg are shown below.

![Jupiter](doc/jupiter.png)

![lander](doc/lander.png)

![mountains](doc/mountains.png)

![bus](doc/school-bus.png)

Thanks to [Freepik](http://www.freepik.com) for the example icons on this page and in the testdata folder.

Icons made by [Freepik](http://www.freepik.com) from [Flaticon](https://www.flaticon.com/) are licensed by [CC 3.0 BY](http://creativecommons.org/licenses/by/3.0/).
