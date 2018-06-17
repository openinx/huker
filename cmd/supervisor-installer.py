#!/usr/bin/python

import subprocess

INSTALL_DIR='.'
INSTALL_PKG='huker-1.0.0.tar.gz'
INSTALL_PKG_URL='http://127.0.0.1:4000/' + INSTALL_PKG
AGNET_PORT=9002

def run_shell(cmd):
	print cmd
	proc = subprocess.Popen(cmd,
						              shell=True,
						              stdout=subprocess.PIPE,
						              stderr=subprocess.PIPE)
	stdout, stderr = proc.communicate()
	print proc.returncode, stdout, stderr
	if proc.returncode == 0:
		return proc.returncode, stdout
	else:
		return proc.returncode, stderr

class HukerAgentInstaller:

	def __init__(self):
		pass

	def start(self):
		ret, err = run_shell('wget %s -O %s/%s' % (INSTALL_PKG_URL, INSTALL_DIR, INSTALL_PKG))
		if ret != 0:
			raise Exception("Failed to download package, %s" % err)
		ret, err = run_shell('tar xzvf %s/%s -C %s' % (INSTALL_DIR, INSTALL_PKG, INSTALL_DIR))
		if ret != 0:
			raise Exception("Failed to unzip the package, %s" % err)
		pkg_dir = INSTALL_PKG
		if pkg_dir.endswith('.tar.gz'):
			pkg_dir = pkg_dir[:-len('.tar.gz')]
		ret, err = run_shell('nohup %s/%s/bin/huker \
			                    --log-level INFO \
			                    --log-file %s/supervisor.log \
									        start-agent \
									        --dir %s \
									        --port %s \
									        --file %s/supervisor.db \
									        >/dev/null 2>&1 &'
													%(INSTALL_DIR,
														pkg_dir,
														INSTALL_DIR,
														INSTALL_DIR,
														AGNET_PORT,
														INSTALL_DIR))
		if ret != 0:
			raise Exception("Failed to start the huker agent.")
		#TODO check the process still alive.

def main():
	installer = HukerAgentInstaller()
	installer.start()

main()
