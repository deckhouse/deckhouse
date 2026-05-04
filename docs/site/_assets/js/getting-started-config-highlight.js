/**
 * YAML snippet legend (mightChange / mustChange) for DVP getting started only.
 * Load before getting-started-dvp-shared.js (DKP uses inline config_highlight in getting-started.js).
 */
function config_highlight() {
  var matchMightChangeEN = /# [Yy]ou might consider changing this\.?/;
  var matchMightChangeRU = /# [Вв]озможно, захотите изменить\.?/;

  function nextCodeSpan($el) {
    var n = $el.next();
    while (n.length && (n.hasClass('line-number') || n.attr('data-copy') === 'ignore')) {
      n = n.next();
    }
    return n;
  }

  function isLineNumberSpan(el) {
    return $(el).hasClass('line-number') || $(el).attr('data-copy') === 'ignore';
  }

  $('code span.c1').filter(function () {
    return matchMightChangeEN.test(this.innerText) || matchMightChangeRU.test(this.innerText);
  }).each(function () {
    var $third = nextCodeSpan(nextCodeSpan(nextCodeSpan($(this))));
    var $colon = $third.length && nextCodeSpan($third).length && nextCodeSpan($third).text().trim() === ':' ? nextCodeSpan($third) : null;
    try {
      if ($third.length && $third.text() === '-') {
        nextCodeSpan($third).addClass('mightChange');
      } else if ($third.length && $third.text().trim() === ':') {
        var $afterColon = nextCodeSpan($third);
        if ($afterColon.length && $afterColon.text().trim() === '' && nextCodeSpan($afterColon).length) {
          nextCodeSpan($afterColon).addClass('mightChange');
        } else if ($afterColon.length) {
          $afterColon.addClass('mightChange');
        }
      } else if ($colon && nextCodeSpan($colon).length && /[\n]/.test(nextCodeSpan($colon).text())) {
        var $cursor = nextCodeSpan($colon);
        while ($cursor.length && /^[\s\n]*$/.test($cursor.text())) {
          $cursor = nextCodeSpan($cursor);
        }
        while ($cursor.length && $cursor.text().indexOf(':') === -1 && $cursor.text().indexOf('\n') === -1) {
          $cursor = nextCodeSpan($cursor);
        }
        if ($cursor.length && $cursor.text().indexOf(':') !== -1) {
          $cursor = nextCodeSpan($cursor);
          while ($cursor.length && /^\s*$/.test($cursor.text()) && $cursor.text().indexOf('\n') === -1 && !isLineNumberSpan($cursor[0])) {
            $cursor = nextCodeSpan($cursor);
          }
          if ($cursor.length && !isLineNumberSpan($cursor[0])) {
            $cursor.addClass('mightChange');
          }
        }
      } else if ($third.length && !isLineNumberSpan($third[0])) {
        $third.addClass('mightChange');
      }
    } catch (e) {
      if ($third.length && !isLineNumberSpan($third[0])) $third.addClass('mightChange');
    }
    $(this).addClass('mightChange');
  });

  $('.language-yaml code span').filter(function () {
    return this.innerText.match('!CHANGE_') ? this.innerText.match('!CHANGE_').length > 0 : false;
  }).each(function () {
    $(this).prev().addClass('mustChange');
    $(this).addClass('mustChange');
  });
}
