go build
(cd control && go build)
(cd control-goccy && go build)
(cd control-jsoniter && go build)
# Fails
# (cd control-pkgjson && go build)

if ! [[ -e "large-file.json" ]]; then
    curl -L -o tmp.json https://raw.githubusercontent.com/json-iterator/test-data/master/large-file.json
    cat tmp.json | jq '.[]' > large-file.json
    rm tmp.json
fi

hyperfine --warmup 2 \
	  --export-markdown basic-benchmark.md \
	  "cat large-file.json | ./jqgo '.repo.url'" \
	  "cat large-file.json | ./control/control '.repo.url'" \
	  "cat large-file.json | ./control-goccy/control '.repo.url'" \
	  "cat large-file.json | ./control-jsoniter/control '.repo.url'" \
	  "cat large-file.json | jq '.repo.url'" \
	  "cat large-file.json | dasel -p json -m '.[*].repo.url'"
          # pkg/json fails
	  # "cat large-file.json | ./control-pkgjson/control '.repo.url'" \

# hyperfine --warmup 2 \
# 	  --export-markdown object-benchmark.md \
# 	  "cat large-file.json | ./jqgo '.payload.release.assets.0.size'" \
# 	  "cat large-file.json | ./control/control '.payload.release.assets.0.size'" \
# 	  "cat large-file.json | jq '.payload.release.assets[0].size'"
