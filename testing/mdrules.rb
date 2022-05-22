all
rule 'MD002', :level => 2
rule 'MD026', :punctuation => ".,;:!"
rule 'MD029', :style => ":ordered"
exclude_rule 'MD013'
# MD022 Headers should be surrounded by blank lines
# Let's think about enabling this later.
exclude_rule 'MD022'
# MD032 - Lists should be surrounded by blank lines
# Excluded because there are no problems with parsers in this case.
exclude_rule 'MD032'
# MD031 - Fenced code blocks should be surrounded by blank lines
# The rule influence some other rules, e.g. MD023 (Headers must start at the beginning of the line).
# Let's try to enable it for new files and see.
# exclude_rule 'MD031'
# MD041 - First line in file should be a top level header
# Excluded because often document contains a front-matter block and it is normal if the document does not contain a top-level header at all.
exclude_rule 'MD041'
