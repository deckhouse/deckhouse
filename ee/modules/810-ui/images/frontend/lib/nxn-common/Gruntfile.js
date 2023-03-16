module.exports = function(grunt) {

  grunt.loadNpmTasks('grunt-webfont');

  grunt.initConfig({
    pkg: grunt.file.readJSON('package.json'),
    webfont: {
      nxn: {
        src: 'icons/src/nxn/*.svg',
        dest: 'icons/build/nxn/fonts',
        destCss: 'icons/build/nxn/css',
        options: {
          font: 'nxn-icons',
          syntax: 'bootstrap',
          htmlDemo: false,
          autoHint: false,
          hashes: false,
          fontPathVariables: true,
          relativeFontPath: '~nxn-common/icons/build/nxn/fonts',
          templateOptions: {
            baseClass: 'nxni',
            classPrefix: 'nxni-',
          }
        }
      }
    }
  });

  grunt.registerTask('build-webfont', ['webfont:nxn']);
};
