// DVP GS step 4: mightChange highlight (fork of DKP config_highlight for .dvp-config-yaml).

// Mark editable YAML values after «Возможно, захотите изменить» comments.
function dvp_config_highlight() {
  let matchMightChangeEN = /# [Yy]ou might consider changing this\.?/;
  let matchMightChangeRU = /# [Вв]озможно, захотите изменить\.?/;

  // Next syntax-highlight span, skip line numbers.
  function nextCodeSpan($el) {
    let n = $el.next();
    while (n.length && (n.hasClass('line-number') || n.attr('data-copy') === 'ignore')) {
      n = n.next();
    }
    return n;
  }

  // Line number or copy-ignore span?
  function isLineNumberSpan(el) {
    return $(el).hasClass('line-number') || $(el).attr('data-copy') === 'ignore';
  }

  // Indent-only span text.
  function isWhitespaceOnly(text) {
    return /^[\t ]*$/.test(text);
  }

  // Span is a line break.
  function isNewlineSpan(text) {
    return text === '\n' || text === '\r\n';
  }

  // Span text contains newline.
  function spanEndsLine(text) {
    return text.indexOf('\n') !== -1;
  }

  // Skip YAML line indent spans.
  function skipLineIndent($cursor) {
    while ($cursor.length && isWhitespaceOnly($cursor.text()) && !spanEndsLine($cursor.text()) && !isLineNumberSpan($cursor[0])) {
      $cursor = nextCodeSpan($cursor);
    }
    return $cursor;
  }

  // Add .mightChange to value spans until EOL.
  function highlightSpansUntilNewline($start) {
    let $cursor = $start;
    while ($cursor.length) {
      if (isLineNumberSpan($cursor[0])) {
        $cursor = nextCodeSpan($cursor);
        continue;
      }
      let text = $cursor.text();
      if (isNewlineSpan(text)) {
        break;
      }
      if (spanEndsLine(text)) {
        $cursor.addClass('mightChange');
        break;
      }
      if (!isWhitespaceOnly(text)) {
        $cursor.addClass('mightChange');
      }
      $cursor = nextCodeSpan($cursor);
    }
  }

  // Highlight value after key: on same line.
  function highlightValueAfterColon($from) {
    let $cursor = $from;
    let text = $cursor.text();
    if (text.trim() === ':') {
      $cursor = nextCodeSpan($cursor);
    } else if (text.indexOf(':') !== -1) {
      $cursor = nextCodeSpan($cursor);
    }
    while ($cursor.length && isWhitespaceOnly($cursor.text()) && !spanEndsLine($cursor.text()) && !isLineNumberSpan($cursor[0])) {
      $cursor = nextCodeSpan($cursor);
    }
    highlightSpansUntilNewline($cursor);
  }

  // Highlight one YAML line (scalar or list item).
  function highlightValueLine($lineStart) {
    let $cursor = skipLineIndent($lineStart);
    if (!$cursor.length || isLineNumberSpan($cursor[0])) {
      return;
    }

    if ($cursor.text().trim() === '-') {
      $cursor = nextCodeSpan($cursor);
      while ($cursor.length && isWhitespaceOnly($cursor.text()) && !spanEndsLine($cursor.text())) {
        $cursor = nextCodeSpan($cursor);
      }
      highlightSpansUntilNewline($cursor);
      return;
    }

    let $scan = $cursor;
    while ($scan.length) {
      if (isLineNumberSpan($scan[0])) {
        $scan = nextCodeSpan($scan);
        continue;
      }
      let text = $scan.text();
      if (isNewlineSpan(text) || spanEndsLine(text)) {
        break;
      }
      if (text.trim() === ':' || (text.indexOf(':') !== -1 && /:\s*$/.test(text))) {
        highlightValueAfterColon($scan);
        return;
      }
      $scan = nextCodeSpan($scan);
    }

    highlightSpansUntilNewline($cursor);
  }

  // After mightChange comment → highlight next value line.
  function highlightValueAfterComment($comment) {
    let $cursor = nextCodeSpan($comment);
    while ($cursor.length) {
      if (isLineNumberSpan($cursor[0])) {
        $cursor = nextCodeSpan($cursor);
        continue;
      }
      let text = $cursor.text();
      if (isNewlineSpan(text)) {
        $cursor = nextCodeSpan($cursor);
        break;
      }
      if (spanEndsLine(text)) {
        $cursor = nextCodeSpan($cursor);
        break;
      }
      $cursor = nextCodeSpan($cursor);
    }

    while ($cursor.length && $cursor.hasClass('c1')) {
      while ($cursor.length) {
        if (isLineNumberSpan($cursor[0])) {
          $cursor = nextCodeSpan($cursor);
          continue;
        }
        let text = $cursor.text();
        if (isNewlineSpan(text)) {
          $cursor = nextCodeSpan($cursor);
          break;
        }
        if (spanEndsLine(text)) {
          $cursor = nextCodeSpan($cursor);
          break;
        }
        $cursor = nextCodeSpan($cursor);
      }
    }

    highlightValueLine($cursor);
  }

  $('code span.c1').filter(function () {
    return (matchMightChangeEN.test(this.innerText)) || (matchMightChangeRU.test(this.innerText));
  }).each(function () {
    $(this).addClass('mightChange');
    highlightValueAfterComment($(this));
  });
}
