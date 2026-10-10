usage: webdownload [options] <weburl> <webdir> <username> [ctlfile]
       webdownload -cleanup <webdir> [username]


This program won't copy more than 5GB of files and checks to make
sure that the destination directory has 20GB free.
Make sure you don't have webfiles directory under your main directory
on the web server; those files can be deleted.