# safeGpx

`safeGpx` is a tool for filtering out unwanted regions from GPX files.

#### Example usage:

<pre>$ safeGpx -skipArea 29.212000,53.136975,29.214000,53.136725 sourceGpxFile.gpx -o outputGpxFile.gpx</pre>

Where `29.212000,53.136975` are coordinates of top-left corner, and `29.214000,53.136725` are coordinates of bottom-right corner of rectangle which should be omitted from output GPX file (i.e. points from this rectangle will be omitted). You are also may specify freeform polygon by providing ≥ 3 points.