const path = require('path')
const glob = require('glob')
const webpack = require('webpack')

const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const PurgecssPlugin = require('purgecss-webpack-plugin')
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
  entry: {
    "status": [
      './status.js',
      './css/main.css'
      ]
  },
  output: {
    filename: 'status.bundle.js',
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
            plugins:["@babel/plugin-proposal-class-properties"]
          }
        }],
      },
      {
        test: /\.css$/,
        use: [
          MiniCssExtractPlugin.loader,
          'css-loader'
        ]
      }
    ],
  },
  plugins: [
    new MiniCssExtractPlugin({
      filename: '[name].css'
    }),
    new PurgecssPlugin({
      paths: glob.sync(`${paths.src}/**/*`,  { nodir: true }),
      safelist: {
        greedy: [/(text|alert)-(success|warning|danger)/]
      }
    }),
    new HtmlWebpackPlugin({
      template: path.resolve(__dirname, "src", "index.html")
    }),
    new AppManifestWebpackPlugin({
      logo: './assets/upmeter-status.png',
      prefix: '',
      output: '/assets/icons-[hash:8]/',
      inject: true,
      emitStats: false,
      config: {
        appName: 'Status',
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
    })
  ],
}

