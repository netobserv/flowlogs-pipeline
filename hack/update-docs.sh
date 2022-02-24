#!/bin/bash

set -eou pipefail

update-readme() {
  # update flowlogs-pipeline command line output
	help=$(./flowlogs-pipeline --help | sed ':a;N;$!ba;s|\n|  \\n|g')
	md_tag=AUTO-flowlogs-pipeline_help
	sed -z -i 's|<!---'$md_tag'--->.*<!---END-'$md_tag'--->|<!---'$md_tag'--->'"\n\`\`\`bash\n$help\n\`\`\`\n"'<!---END-'$md_tag'--->|g' README.md

  # update makefile help output
	help=$(make --no-print-directory help | sed ':a;N;$!ba;s|\n|  \\n|g' | sed -r 's/[[:cntrl:]]\[[0-9]{1,3}m//g' )
	md_tag=AUTO-makefile_help
	sed -z -i 's|<!---'$md_tag'--->.*<!---END-'$md_tag'--->|<!---'$md_tag'--->'"\n\`\`\`bash\n$help\n\`\`\`\n"'<!---END-'$md_tag'--->|g' README.md
}

main() {
  update-readme
}

main
