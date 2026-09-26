# Sourced by backup.sh / restore.sh (and Server/app's Makefile). Loads a
# docker-compose-style .env file as DATA: KEY=VALUE lines are exported
# verbatim (surrounding quotes stripped), nothing is executed. Sourcing the
# file as shell instead breaks on ordinary values — e.g.
# MAIL_FROM=Bibliomania <no-reply@…> is a shell redirect.
load_env() {
  local file=$1 line key value
  while IFS= read -r line || [[ -n $line ]]; do
    [[ $line =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    key=${BASH_REMATCH[1]}
    value=${BASH_REMATCH[2]}
    # Strip a trailing " # comment" only when the value isn't quoted.
    if [[ $value =~ ^\"(.*)\"$ || $value =~ ^\'(.*)\'$ ]]; then
      value=${BASH_REMATCH[1]}
    else
      value=${value%%[[:space:]]#*}
      value=${value%"${value##*[![:space:]]}"}
    fi
    export "$key=$value"
  done < "$file"
}
