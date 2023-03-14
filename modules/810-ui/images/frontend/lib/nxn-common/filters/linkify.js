import linkifyHtml from 'linkify-html';

export default function(text) {
  return linkifyHtml(text || '', { defaultProtocol: 'https', target: '_blank' });
}
