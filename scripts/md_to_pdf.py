#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# dependency: wkhtmltopdf (http://wkhtmltopdf.org)
# inspired: https://github.com/revolunet/sublimetext-markdown-preview

import os
import sys
import traceback
import json
import cgi
from urllib.request import urlopen
from urllib.error import HTTPError
import urllib.request
from subprocess import call
from os.path import expanduser


def get_contents(filepath):
    f = open(filepath, 'r')
    data = f.read()
    f.close()
    return data


def md_to_html_body(markdown_text):
    print('converting markdown with github API...')
    data = {
        "text": markdown_text,
        "mode": "gfm"
    }
    data = json.dumps(data).encode('utf-8')

    markdown_html = ''
    try:
        headers = {
            'Content-Type': 'application/json'
        }
        url = "https://api.github.com/markdown"
        request = urllib.request.Request(url, data, headers, 'POST')
        markdown_html = urlopen(request).read().decode('utf-8')

    except HTTPError:
        e = sys.exc_info()[1]
        if e.code == 401:
            print('github API auth failed. Please check your OAuth token.')
        else:
            print('github API responded in an unfriendly way :/')
    except:
        e = sys.exc_info()[1]
        print(e)
        traceback.print_exc()
        print('cannot use github API to convert markdown.')
    else:
        print('converted markdown with github API successfully')

    return markdown_html


def get_css(css_filepath):
    css_path = os.path.dirname(os.path.abspath(__file__))
    css_path += '/detail/github.css'
    return '<style>%s</style>' % get_contents(css_path)


def get_title(filepath):
    title = os.path.splitext(os.path.basename(filepath))[0]
    return '<title>%s</title>' % cgi.escape(title)


def make_html(html_path, md_html_body):
    html = u'<!DOCTYPE html>'
    html += '<html><head><meta charset="utf-8">'
    html += get_css('github.css')
    html += get_title(html_path)
    html += '</head><body>'
    html += '<article class="markdown-body">'
    html += md_html_body
    html += '</article>'
    html += '</body>'
    html += '</html>'
    f = open(html_path, 'w')
    f.write(html)
    f.close()


def main(md_path, pdf_path):
        home = expanduser("~") + '/'
        html_path = home + os.path.splitext(os.path.basename(md_path))[0]
        html_path += '.html'
        md_text = get_contents(md_path)
        html = md_to_html_body(md_text)
        make_html(html_path, html)
        print("created html file (%s)" % html_path)
        call(["wkhtmltopdf", html_path, pdf_path])
        os.remove(html_path)
        print("%s to %s completed." % (md_path, pdf_path))


if __name__ == '__main__':
    if not len(sys.argv) == 3:
        print("Usage: ./md_to_pdf.py [source md file] [target pdf file]")
        sys.exit()
    main(sys.argv[1], sys.argv[2])
