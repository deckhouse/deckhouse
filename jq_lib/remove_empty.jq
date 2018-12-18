def isempty:
  . == {} or . == [] or . == null;

def remove_empty:
  . as $in |

  if isempty then del(.) else
    if type == "object" then
      # let's make a new object with empty or null values removed
      reduce keys[] as $key (
        {}; . +
          (($in[$key] | remove_empty) as $result
          | if $result then
            { ($key):  ($result) } else {} end )
      )
    # let's make a new array with empty or null elements removed
    elif type == "array" then map(remove_empty)
    else .
    end
    # if the result is empty or null - discard it completely
    | select(isempty | not)
  end;
