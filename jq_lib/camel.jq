def camel:
  gsub("-(?<a>[a-z])"; .a|ascii_upcase);
