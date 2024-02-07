const path = require("path")
const glob = require("glob")
const webpack = require("webpack")

const MiniCssExtractPlugin = require("mini-css-extract-plugin")
const PurgecssPlugin = require("purgecss-webpack-plugin")
const CssMinimizerPlugin = require("css-minimizer-webpack-plugin")
const HtmlWebpackPlugin = require("html-webpack-plugin")
const AppManifestWebpackPlugin = require("app-manifest-webpack-plugin")
const ForkTsCheckerWebpackPlugin = require("fork-ts-checker-webpack-plugin")
const BabelMinifyPlugin = require("babel-minify-webpack-plugin")

const paths = {
  modules: path.join(__dirname, "modules"),
  src: path.join(__dirname, "src"),
  dist: path.join(__dirname, "dist"),
  data: path.join(__dirname, "data"),
  vendor: path.join(__dirname, "vendor"),
}

module.exports = {
  mode: "production",
  devtool: "source-map",

  context: paths.src,
  entry: {
    upmeter: ["./app/index.tsx"],
  },
  output: {
    filename: "upmeter.bundle.js",
    path: paths.dist,
    publicPath: "",
  },
  module: {
    rules: [
      {
        test: /\.jsx?$/,
        exclude: [/node_modules/],
        use: [
          {
            loader: "babel-loader",
            options: {
              presets: [["@babel/preset-env", { targets: "defaults" }], "@babel/preset-react"],
              plugins: ["@babel/plugin-proposal-class-properties"],
            },
          },
        ],
      },
      {
        test: /\.tsx?$/,
        exclude: /node_modules/,
        use: [
          {
            loader: "babel-loader",
            options: {
              cacheDirectory: true,
              babelrc: false,
              // Note: order is top-to-bottom and/or left-to-right
              plugins: [
                [
                  require("@rtsao/plugin-proposal-class-properties"),
                  {
                    loose: true,
                  },
                ],
                "@babel/plugin-proposal-nullish-coalescing-operator",
                "@babel/plugin-proposal-optional-chaining",
                "@babel/plugin-syntax-dynamic-import", // needed for `() => import()` in routes.ts
              ],
              // Note: order is bottom-to-top and/or right-to-left
              presets: [
                [
                  "@babel/preset-env",
                  {
                    targets: {
                      browsers: "last 3 versions",
                    },
                    useBuiltIns: "entry",
                    corejs: 3,
                    modules: false,
                  },
                ],
                [
                  "@babel/preset-typescript",
                  {
                    allowNamespaces: true,
                  },
                ],
                "@babel/preset-react",
              ],
            },
          },
        ],
      },
      {
        test: /\.css$/,
        use: [
          MiniCssExtractPlugin.loader,
          {
            loader: "css-loader",
            options: { sourceMap: true },
          },
        ],
      },
      {
        test: /\.scss$/,
        use: [
          MiniCssExtractPlugin.loader,
          {
            loader: "css-loader",
            options: { sourceMap: true },
          },
          {
            loader: "sass-loader",
            options: { sourceMap: true },
          },
        ],
      },
      {
        test: /\.(png|jpg|gif|ttf|eot|svg|woff(2)?)(\?[a-z0-9=&.]+)?$/,
        loader: "file-loader",
      },
    ],
  },
  resolve: {
    extensions: [".tsx", ".ts", ".jsx", ".js"],
    alias: {
      moment: path.resolve(path.join(__dirname, "node_modules", "moment")),
    },
    modules: [path.resolve(__dirname, "src", "modules"), "node_modules"],
  },
  optimization: {
    nodeEnv: "production",
    minimize: true,
    minimizer: [
      new BabelMinifyPlugin(),
      new CssMinimizerPlugin({
        minimizerOptions: {
          sourceMap: true,
          preset: [
            "default",
            {
              discardComments: { removeAll: true },
            },
          ],
        },
      }),
    ],
  },
  plugins: [
    new webpack.ContextReplacementPlugin(/moment[/\\]locale$/, /ru/),
    new ForkTsCheckerWebpackPlugin({
      typescript: {
        configFile: path.join(__dirname, "tsconfig.json"),
      },
    }),
    new MiniCssExtractPlugin({
      filename: "[name].css",
    }),
    new PurgecssPlugin({
      paths: glob.sync(`${paths.src}/**/*`, { nodir: true }),
      safelist: {
        greedy: [/popper|pie|top-tick|graph-|group-|probe-|cell-/],
      },
    }),
    new HtmlWebpackPlugin({
      template: path.resolve(__dirname, "src", "index.html"),
    }),
    new AppManifestWebpackPlugin({
      logo: "./assets/upmeter.png",
      prefix: "",
      output: "/assets/icons-[hash:8]/",
      inject: true,
      emitStats: false,
      config: {
        appName: "Upmeter",
        icons: {
          android: false,
          appleIcon: false,
          appleStartup: false,
          coast: false,
          favicons: true,
          firefox: false,
          windows: false,
          yandex: false,
        },
      },
    }),
    function () {
      this.hooks.done.tap("Done", function (stats) {
        if (stats.compilation.errors && stats.compilation.errors.length) {
          console.log(stats.compilation.errors)
          process.exit(1)
        }
      })
    },
  ],
}
