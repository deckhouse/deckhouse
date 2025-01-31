## regcopy

A tool to sanitize and copy base images to the Deckhouse container registry.

Usage: 
```sh
regcopy <image>:<tag>
go run . <image>:<tag>
```

### Purpose

* Copy an image from a public registry to the Deckhouse registry.
* Remove all labels from an image.

### Why do not use crane/skopeo/etc.?

1. `regcopy` is a script based on the [`go-containerregistry`](https://github.com/google/go-containerregistry) library, 
on which crane is also based. Basically, it works the same way as a `crane copy`, but allows mutations.

2. Instead of using a bash script with a generic tool written in go, it is possible to use just a script in go and a library.
