# mir-sample

This is a small sample application utilizing the [MirBFT library](https://github.com/IBM/mirbft).

**WARNING**  This sample is currently _barely_ functional, and is meant to be instructive, not useful.  **WARNING**

That being said, if you'd like to use it, you may do the following:

1. Execute the cryptogen binary:
 
```
cd cryptogen
go run main.go 4
```

You should now see 4 `config*.yaml` files in your cryptogen directory.

2. Build the `mir-sample` binary.  From the top level directory execute:

```
go build .
```

You should now have a `mir-sample` binary in your top level directory.

3. Execute the start script:

```
./start.sh
```

If all went well, in your `output` directory, you should now have four `*.log` files, numbered 0 through 3, as well as four corresponding `*.eventlog` files.  If you inspect the `*.log` file, you should see scrolling messages like:

```
Applying reqNo=5 with data data-5 to log
Applying reqNo=6 with data data-6 to log
Applying reqNo=7 with data data-7 to log
Applying reqNo=8 with data data-8 to log
Applying reqNo=9 with data data-9 to log
Applying reqNo=10 with data data-10 to log
Applying reqNo=11 with data data-11 to log
Applying reqNo=12 with data data-12 to log
Applying reqNo=13 with data data-13 to log
Applying reqNo=14 with data data-14 to log
```

The `*.eventlog` files can be viewed using either the `mircat` or the `mirbft-visualizer` tools.

Note.  As warned, there are currently unsquashed bugs, and unimplemented features, and it's been observed that this network will often break and stop working after some period of time.  That's expected, please don't report it as a bug.  Feel free to submit PRs for documentation improvements, usability improvements, etc. but work to find and address these bugs is underway in the main repository with automated testing frameworks.
