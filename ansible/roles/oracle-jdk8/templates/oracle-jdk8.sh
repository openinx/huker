#!/usr/bin/env bash

JDK_PATH={{ huker_install_dir }}/oracle-jdk8
mkdir -p {{ huker_install_dir }}/oracle-jdk8
tar xzvf {{ huker_install_dir }}/oracle-jdk8.tar.gz -C $JDK_PATH

for subDir in $(ls $JDK_PATH); do
    if [[ -d $JDK_PATH/$subDir ]] ; then
      if [[ -d $JDK_PATH/$subDir/bin ]]; then
        if [[ -f $JDK_PATH/$subDir/bin/java ]]; then
          ln -nsf $JDK_PATH/$subDir/bin/java  /usr/local/bin/java
        fi
      fi
    fi
done
