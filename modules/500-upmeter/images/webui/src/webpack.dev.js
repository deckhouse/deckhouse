const path = require("path")
const webpack = require("webpack")

//const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackPlugin = require("html-webpack-plugin")
const AppManifestWebpackPlugin = require("app-manifest-webpack-plugin")
const { CleanWebpackPlugin } = require("clean-webpack-plugin")
const ForkTsCheckerWebpackPlugin = require("fork-ts-checker-webpack-plugin")

const paths = {
  src: path.join(__dirname, "src"),
  dist: path.join(__dirname, "dist"),
  data: path.join(__dirname, "data"),
  vendor: path.join(__dirname, "vendor"),
}

module.exports = {
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
      // {
      //   // Exclude monaco-editor comes from @grafana/ui
      //   test: /node_modules\/(react-)?monaco-editor/,
      //   use: 'null-loader',
      // },
      {
        test: /\.jsx?$/,
        exclude: [/node_modules/],
        use: [
          {
            loader: "babel-loader",
            options: {
              presets: [["@babel/preset-env", { targets: "defaults" }], "@babel/preset-react"],
              //plugins: ['@babel/plugin-transform-runtime'],
              plugins: ["@babel/plugin-proposal-class-properties"],
            },
          },
        ],
      },
      {
        test: /\.tsx?$/,
        exclude: [/node_modules/],
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
        use: ["style-loader", "css-loader"],
      },
      {
        test: /\.scss$/,
        use: ["style-loader", "css-loader", "sass-loader"],
      },
      {
        test: /\.(png|jpg|gif|ttf|eot|svg|woff(2)?)(\?[a-z0-9=&.]+)?$/,
        loader: "file-loader",
      },
    ],
  },
  resolve: {
    extensions: [".tsx", ".ts", ".jsx", ".js"],
    modules: [path.resolve(__dirname, "src", "modules"), "node_modules"],
  },
  devServer: {
    contentBase: paths.dist,
    compress: true,
    port: "4800",
    stats: "errors-only",
    proxy: {
      "/api": {
        target: "http://localhost:8091",
        logLevel: "debug",
      },
    },
  },
  devtool: "#inline-source-map",
  plugins: [
    new CleanWebpackPlugin(),
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
    new webpack.DefinePlugin({
      "process.env": {
        NODE_ENV: JSON.stringify("development"),
      },
    }),
  ],
}
