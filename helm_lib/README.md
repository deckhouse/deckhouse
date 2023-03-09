ATTENTION! DO NOT CHANGE FILES IN `helm_lib` DIRECTORY DIRECTLY!

Use https://github.com/deckhouse/lib-helm#working-with-repo instruction to change `lib-helm`
and update `lib-helm` in Deckhouse repo:

```bash
make version=DESIRED_VERSION update-lib-helm
```

For example:
```bash
make version=0.0.2 update-lib-helm
```
