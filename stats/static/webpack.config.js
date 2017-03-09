const path = require('path');
const webpack = require('webpack');
const pkg = require('./package.json');
const ExtractTextPlugin = require('extract-text-webpack-plugin');

const isProd = process.env.NODE_ENV === 'production';

module.exports = {
  devtool: isProd ? '' : 'sourcemap',
  context: path.join(__dirname, 'src'),
  entry: {
    app: ['./app.js'],
  },
  output: {
    path: path.resolve(pkg.config.buildDir),
    publicPath: '/',
    filename: '[name].js',
  },
  module: {
    loaders: [
      {
        test: /\.(woff|woff2)(\?v=\d+\.\d+\.\d+)?$/,
        loader: 'url-loader?limit=5000&mimetype=application/font-woff&prefix=fonts',
      },
      {
        test: /\.ttf(\?v=\d+\.\d+\.\d+)?$/,
        loader: 'url-loader?limit=5000&mimetype=application/octet-stream&prefix=fonts',
      },
      {
        test: /\.eot(\?v=\d+\.\d+\.\d+)?$/,
        loader: 'url-loader?limit=5000&mimetype=application/vnd.ms-fontobject&prefix=fonts',
      },
      {
        test: /\.css$/,
        loader: ExtractTextPlugin.extract({ fallback: 'style-loader', use: ['css-loader?importLoaders=1', 'postcss-loader'] }),
      },
      {
        test: /\.js$/,
        exclude: /node_modules/,
        loader: 'babel-loader',
      },
      {
        test: /\.html$/,
        exclude: /node_modules/,
        loader: 'file-loader?name=[path]index.[ext]',
      },
      {
        test: /\.(jpe?g|png|gif|svg)$/i,
        loaders: [
          'file-loader?hash=sha512&digest=hex&name=[path][hash].[ext]',
          'image-webpack-loader?{progressive:true, optimizationLevel: 7, interlaced: false, pngquant:{quality: "65-90", speed: 4}}',
        ],
      },
    ],
  },
  plugins: [
    new webpack.NoEmitOnErrorsPlugin(),
    new ExtractTextPlugin({
      filename: '[name].css',
      publicPath: '/',
      allChunks: true,
    }),
    new webpack.DefinePlugin({
      'process.env': {
        NODE_ENV: JSON.stringify(isProd ? 'production' : 'development'),
      },
    }),
    new webpack.ProvidePlugin({
      'es6-promise': 'es6-promise',
    }),
  ],
};
