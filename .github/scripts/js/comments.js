/**
 * Bot related functions.
 */

const WORKFLOW_START_MARKER = '<!-- workflow_start -->'
module.exports.WORKFLOW_START_MARKER = WORKFLOW_START_MARKER;

// Confirm recognition of the user command.
module.exports.commentCommandRecognition = (userName, command) => {
  return `Aye, aye, @${userName}. I've recognized your '${command}' command and started the workflow...\n${WORKFLOW_START_MARKER}`;
};

// Confirm label.
module.exports.commentLabelRecognition = (userName, label) => {
  return  `Aye, aye, @${userName}. I've started the workflow for label '${label}'...\n${WORKFLOW_START_MARKER}`;
};

module.exports.deleteBotComment = (text) => {
  return text.replace(/^Aye,.*\n/m, '');
}

module.exports.releaseIssueHeader = (context, gitRefInfo) => {
  let header = '';
  if (gitRefInfo.isTag) {
    header = `New tag **${gitRefInfo.tagName}** is created.\n${WORKFLOW_START_MARKER}`;
  }
  if (gitRefInfo.isBranch) {
    const commitMiniSHA = context.payload.head_commit.id.slice(0, 6);
    const commitUrl = context.payload.head_commit.url;
    const commitInfo = `New commit [${commitMiniSHA}](${commitUrl}) in branch **${gitRefInfo.branchName}**:`;
    // Format commit message.
    const mdCodeMarker = '```';
    const commitMsg = `${mdCodeMarker}\n${context.payload.head_commit.message}\n${mdCodeMarker}`;
    header = `${commitInfo}\n${commitMsg}\n${WORKFLOW_START_MARKER}`;
  }
  return header;
}

module.exports.commentJobStarted = (jobName, ref, buildUrl) => {
  return `:fast_forward:\u00a0\`${jobName}\` for \`${ref}\` [started](${buildUrl}).`
}

module.exports.deleteJobStartedComments = (jobsReport) => {
  return jobsReport.replace(/^.*:fast_forward:.*started.*\.\n\n?/gm, '');
}

const jobResultMarker = (name) => {
  return `<!-- result-for: ${name} -->`
}

module.exports.hasJobResult = (comment, name) => {
  return comment.includes(jobResultMarker(name));
}



module.exports.renderJobStatusOneLine  = (status, name, started_at) => {
  const time_elapsed = getTimeElapsedForStatus(started_at);
  let statusComment = `:white_check_mark:\u00a0\`${name}\` succeeded${time_elapsed}`;
  if (status === 'failure') {
    statusComment = `:x:\u00a0\`${name}\` failed${time_elapsed}`;
  }
  if (status === 'cancelled') {
    statusComment = `:white_small_square:\u00a0\`${name}\` cancelled`;
  }
  if (status === 'skipped') {
    statusComment = `:white_small_square:\u00a0\`${name}\` skipped`;
  }

  return `${statusComment}.${jobResultMarker(name)}`;
};

module.exports.renderJobStatusSeparate = (status, name, started_at) => {
  const time_elapsed = getTimeElapsedForStatus(started_at);
  let statusComment = `:green_circle:\u00a0\`${name}\` succeeded${time_elapsed}`;
  if (status === 'failure') {
    statusComment = `:red_circle:\u00a0\`${name}\` failed${time_elapsed}`;
  }
  if (status === 'cancelled') {
    statusComment = `:white_circle:\u00a0\`${name}\` cancelled`;
  }

  return `\n${statusComment}.${jobResultMarker(name)}\n`;
};

module.exports.renderWorkflowStatusFinal = (status, name, ref, build_url, started_at) => {
  const time_elapsed = getTimeElapsedForStatus(started_at);
  let statusComment = `:green_circle:\u00a0\`${name}\` for \`${ref}\` [succeeded](${build_url})${time_elapsed}.`;
  if (status === 'failure') {
    statusComment = `:red_circle:\u00a0\`${name}\` for \`${ref}\` [failed](${build_url})${time_elapsed}.`;
  }
  if (status === 'cancelled') {
    statusComment = `:white_circle:\u00a0\`${name}\` for \`${ref}\` [cancelled](${build_url}).`;
  }
  if (status === 'skipped') {
    statusComment = `:white_circle:\u00a0\`${name}\` for \`${ref}\` [skipped](${build_url}).`;
  }

  return statusComment;
};

/**
 * Return a human-readable duration.
 *
 * TODO Consider using a well-known library, e.g. https://date-fns.org/v2.28.0/docs/formatDistanceStrict
 *
 * @param duration_seconds - Duration in seconds.
 * @returns {string}
 */
const humanDuration = (duration_seconds) => {
  let res = '';

  const d = parseInt(duration_seconds, 10);

  // Seconds
  const s = d % 60;
  res = `${s}s`;

  if (d >= 60) {
    // Minutes
    const m = ((d - s) / 60) % 60;
    res = `${m}m${res}`;

    if (d >= 3600) {
      // Hours
      const h = Math.floor(d / 3600);
      res = `${h}h${res}`;
    }
  }

  return res;
};

/**
 * Return a human-readable duration between started_at timestamp and Date.now().
 *
 * @param started_at - A Unix timestamp.
 * @returns {string}
 */
const getTimeElapsedForStatus = (started_at) => {
  if (!started_at) {
    console.log('No started_at time.');
    return '';
  }

  const start_seconds = parseInt(started_at, 10);

  const start_date = new Date();
  start_date.setTime(start_seconds * 1000);

  const now_date = new Date();
  const now_seconds = Math.floor(now_date.getTime() / 1000);

  const duration_seconds = now_seconds - start_seconds;
  const duration_human = humanDuration(duration_seconds);

  console.log(`started_at: ${start_seconds} '${start_date}'`);
  console.log(`now:        ${now_seconds} '${now_date}'`);
  console.log(`duration:   ${duration_seconds} '${duration_human}'`);

  // Return string to embed in status.
  return ` in ${duration_human}`;
}
