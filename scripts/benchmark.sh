go build
(cd control && go build)

if ! [[ -e "large-file.json" ]]; then
    curl -L -o tmp.json https://raw.githubusercontent.com/json-iterator/test-data/master/large-file.json
    cat tmp.json | jq '.[]' > large-file.json
    rm tmp.json
fi

# hyperfine --warmup 2 \
# 	  --export-markdown basic-benchmark.md \
# 	  "cat large-file.json | ./jqgo '.created_at'" \
# 	  "cat large-file.json | ./control/control '.created_at'" \
# 	  "cat large-file.json | jq '.created_at'"

hyperfine --warmup 2 \
	  --export-markdown object-benchmark.md \
	  "cat large-file.json | ./jqgo '.payload.release.assets.0.size'" \
	  "cat large-file.json | ./control/control '.payload.release.assets.0.size'" \
	  "cat large-file.json | jq '.payload.release.assets[0].size'"
