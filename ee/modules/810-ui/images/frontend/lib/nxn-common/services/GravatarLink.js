import md5 from 'md5';

export default function GravatarLink(email, size, def, rating) {
  var hash = (typeof email === 'string') && md5(email.trim().toLowerCase());
  return `//www.gravatar.com/avatar/${hash}?s=${size || 50}&d=${def || 'retro'}&r=${rating || 'g'}`;
}