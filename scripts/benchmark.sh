go build
(cd control && go build)

if ! [[ -e "large-file.json" ]]; then
    curl -L -o tmp.json https://raw.githubusercontent.com/json-iterator/test-data/master/large-file.json
    cat tmp.json | jq '.[]' > large-file.json
    rm tmp.json
fi

# Using cat

hyperfine --warmup 2 \
	  --export-markdown cat-benchmark.md \
	  "cat large-file.json | ./jqgo '.created_at'" \
	  "cat large-file.json | ./control/control '.created_at'" \
	  "cat large-file.json | jq '.created_at'"

# Using gunzip
gzip -k large-file.json

hyperfine --warmup 2 \
	  --export-markdown gunzip-benchmark.md \
	  "gunzip -c large-file.json.gz | ./jqgo '.created_at'" \
	  "gunzip -c large-file.json.gz | ./control/control '.created_at'" \
	  "gunzip -c large-file.json.gz | jq '.created_at'"
