# Load testing

> For tests use LB in one availability zone, because yandex-tank uses only the first address from zone records. To test multiple zone LB you need to specify all LB addresses in `load.yaml` according to ["Multi-tests" section of yandex-tank's documentation](https://yandextank.readthedocs.io/en/latest/core_and_modules.html#multi-tests).
1. _(Optionally)_ Get the token from [Overload](https://overload.yandex.net/) (click your profile photo -> My api token) into a `token.txt` file. And uncomment `overload` section in `load.yaml`.
2. Set LB address in `load.yaml` `phantom.address` parameter.
3. Start the container.

   ```shell
   docker run -v "$PWD:/var/loadtest" -it direvius/yandex-tank
   ```

4. Follow the given `Web:` url to see online graphs.
