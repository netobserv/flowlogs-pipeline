#!/usr/bin/env bash

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

update-animated-gif() {
  # update animated-gif in README.md from all images under docs/images/animated-gif-images
  IMAGES_PATH=$PWD/docs/images
  if [[ $(type -P "podman") ]]; then
    # for podman with selinux, volume mounting requires :Z flag
    podman run -v "$IMAGES_PATH":/docs/images:Z docker.io/dpokidov/imagemagick -loop 0 -delay 500 -resize 800x800 /docs/images/animated-gif-images/*.png /docs/images/animation.gif
  else
    docker run -v "$IMAGES_PATH":/docs/images docker.io/dpokidov/imagemagick -loop 0 -delay 500 -resize 800x800 /docs/images/animated-gif-images/*.png /docs/images/animation.gif
  fi
}


main() {
  update-readme
#  update-animated-gif
}

main
