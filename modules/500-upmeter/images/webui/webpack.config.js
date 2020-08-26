const path = require('path')
const webpack = require('webpack')

const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const AppManifestWebpackPlugin = require('app-manifest-webpack-plugin')

const paths = {
  src: path.join(__dirname, 'src'),
  dist: path.join(__dirname, 'dist'),
  data: path.join(__dirname, 'data'),
  vendor: path.join(__dirname, 'vendor')
}

module.exports = {
  context: paths.src,
  entry: ['./upmeter.js'],
  output: {
    filename: 'upmeter.bundle.js',
    path: paths.dist,
    publicPath: '',
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: [/node_modules/],
        use: [{
          loader: 'babel-loader',
          options: {
            presets: [['@babel/preset-env',{ "targets": "defaults" }]],
            //plugins: ['@babel/plugin-transform-runtime'],
            plugins:["@babel/plugin-proposal-class-properties"]
          }
        }],
      },
      {
        test: /\.css$/,
        //use: [MiniCssExtractPlugin.loader, 'css-loader']
        use: ['style-loader', 'css-loader']
      }
    ],
  },
  devServer: {
    contentBase: paths.dist,
    compress: true,
    port: '4800',
    stats: 'errors-only',
    proxy: {
      '/api': {
        target: 'http://localhost:8091',
        logLevel: 'debug'
      }
    }
  },
  devtool: "#inline-source-map",
  plugins: [
    // new ExtractTextPlugin({
    //   filename: 'main.bundle.css',
    //   allChunks: true,
    // }),
    new CopyWebpackPlugin({
      patterns: [
        {from: paths.vendor+'/*.css', to: paths.dist + '/vendor' }
    ]}),
    new MiniCssExtractPlugin(),
    new HtmlWebpackPlugin({
      template: path.resolve(__dirname, "src", "index.html")
    }),
    new AppManifestWebpackPlugin({
      logo: './assets/upmeter.png',
      prefix: '',
      output: '/assets/icons-[hash:8]/',
      inject: true,
      emitStats: false,
      config: {
        appName: 'Upmeter',
        icons: {
          android: false,
          appleIcon: false,
          appleStartup: false,
          coast: false,
          favicons: true,
          firefox: false,
          windows: false,
          yandex: false,
        }
      }
    }),
    new webpack.ProvidePlugin({
      $: 'jquery',
      jQuery: 'jquery',
      'window.$': 'jquery',
      'window.jQuery': 'jquery',
    })


  ],
}
