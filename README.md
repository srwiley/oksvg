# oksvg
oksvg is a rasterizer for a partial implementation of the SVG2.0 specification in golang.

Although many SVG elements will not be read by oksvg, it is good enough to faithfully produce thousands, but certainly not all, SVG icons available both for free and commercially. A list of valid and invalid elements is in the doc folder.
oksvg uses the [rasterx](https://github.com/srwiley/rasterx) to fully implement SVG2.0 path commands including 'arc' join mode.

![Jupiter](doc/jupiter.png)


### Extra SVG2.0 features.

In addition to 'arc' as a valid join mode value, oksvg also allows 'arc-clip' which is the arc analog of miter-clip and some extra capping and gap values.

![arcs and caps](doc/TestShapes.png)

![lander](doc/lander.png)

Thanks to [Freepik](http://www.freepik.com) from [Flaticon](https://www.flaticon.com/)
Licensed by [Creative Commons BY 3.0](http://creativecommons.org/licenses/by/3.0/) for the example Jupiter and Lander SVG icons and additional icons in the testdata folder.
