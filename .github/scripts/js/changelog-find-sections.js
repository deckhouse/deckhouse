//@ts-check
const fs = require('fs');

// Defined behavior to use
//
// const findSections = require('./.github/scripts/js/changelog-find-sections.js')
module.exports = function findSections() {
  const roots = [
    //
    '.',
    'modules',
    'ee/modules',
    'ee/fe/modules'
  ];

  const exclusions = [
    '^\\.', //    dotfiles
    'CHANGELOG',
    'ee', //      found in "."
    'modules' //  found in "."
  ];

  return find(roots, exclusions);
};

// For testing in Node REPL
// const { find } = require('./.github/scripts/js/changelog-find-sections.js')
module.exports.find = find;

/**
 * Find all supported section names for changelog
 * @param {string[]} roots      the array of directories, e.g. [".", "modules", "ee/modules", "ee/fe/modules"]
 * @param {string[]} exclusions the array of sections to exclude, e.g. ["^\\.", "CHANGELOG", "ee", "modules"]
 * @returns the array of sections
 * call([".", "modules", "ee/modules", "ee/fe/modules"], ["^\\.", "CHANGELOG", "^ee$", "^modules$"])
 */
function find(roots = [], exclusions = []) {
  const foundSections = roots.flatMap((root) => getSections(root, exclusions));
  const uniqSections = new Set(foundSections);
  return Array.from(uniqSections).sort();
}

// returns sections excluding items with given regexes
function getSections(root, exclusions = []) {
  // trim numbers
  const sections = getSubdirs(root).map((name) => name.replace(/^\d+-/g, ''));

  if (exclusions.length == 0) {
    return sections;
  }

  const shouldExclude = (name) => exclusions.some((pat) => new RegExp(pat).test(name));
  return sections.filter((s) => !shouldExclude(s));
}

// returns dir names in a given root dir
function getSubdirs(root) {
  return fs
    .readdirSync(root, { withFileTypes: true })
    .filter((d) => d.isDirectory())
    .map((d) => d.name);
}
