package crawler

import (
	"strings"
	"testing"
)

func TestExtractAnnouncementLinks(t *testing.T) {
	html := `
	<ul>
	  <li><a href="./zkgg/202601/t20260112_1.shtml">湖北省2026年度省市县乡考试录用公务员公告2026-01-12</a></li>
	  <li><a href="http://60.205.190.64:84">湖北省2026年度市州级以上机关公开遴选公务员报名入口</a></li>
	  <li><a href="/bmdt/tzgg/index.html">事业单位考试</a></li>
	  <li><a href="./zytz/202606/t20260616_2.shtml">湖北省卫生健康委2026年度考试录用公务员拟录用人员公示</a></li>
	</ul>`
	links := extractAnnouncementLinks(html, "https://rst.hubei.gov.cn/hbrsksw/sy.shtml")
	if len(links) != 2 {
		t.Fatalf("links %d %+v", len(links), links)
	}
	if links[0].href != "https://rst.hubei.gov.cn/hbrsksw/zkgg/202601/t20260112_1.shtml" {
		t.Fatalf("href %s", links[0].href)
	}
}

func TestExtractHunanAnnouncementLinks(t *testing.T) {
	html := `
	<li><a title='湖南省2027年定向选调公告' target="_blank" href="/rst/hnrsksw/c103103/gwy2026/202609/t20260918_34067735.html"><small>2026-09-18</small><span>湖南省2027年定向选调公告</span></a></li>
	<li><a title='湖南省2027年定向选调报名入口' target="_blank" href="https://rsks.onlydwy.com/current/exams?tab=0"><span>报名入口</span></a></li>
	<li><a href="/rst/xxgk/zpzl/sydwzp/202609/t20260908_34059489.html" target="_blank">湖南省2026年省直事业单位第四次公开招聘工作人员公告</a></li>
	<li><a href="https://www.hxw.gov.cn/content/2026/09/02/14931618.html" target="_blank">2026年度湖南省省直机关公开遴选公务员公告</a></li>
	<li><a title='湖南省2026年补充录用公务员公告' href="https://www.hxw.gov.cn/content/2026/05/14/14895048.html?" target="_blank"><span>湖南省2026年补充录用公务员公告</span></a></li>`
	links := extractAnnouncementLinks(html, "https://rst.hunan.gov.cn/rst/hnrsksw/c103103/gwy2026/rskswlist.html")
	if len(links) != 4 {
		t.Fatalf("links %d %+v", len(links), links)
	}
	if links[0].title != "湖南省2027年定向选调公告" {
		t.Fatalf("title %q", links[0].title)
	}
	if links[0].href != "https://rst.hunan.gov.cn/rst/hnrsksw/c103103/gwy2026/202609/t20260918_34067735.html" {
		t.Fatalf("href %s", links[0].href)
	}
	if links[1].href != "https://rst.hunan.gov.cn/rst/xxgk/zpzl/sydwzp/202609/t20260908_34059489.html" {
		t.Fatalf("sydw href %s", links[1].href)
	}
	if links[3].href != "https://www.hxw.gov.cn/content/2026/05/14/14895048.html" {
		t.Fatalf("query href %s", links[3].href)
	}
}

func TestStripTagsDropsScript(t *testing.T) {
	in := `<title>湖南省2027年定向选调公告</title><script>$(document).ready(function(){ var url = location.href; })</script><div>根据选调生工作的规定公告如下。</div>`
	got := stripTags(in)
	if strings.Contains(got, "document") || strings.Contains(got, "location") {
		t.Fatalf("script leaked: %q", got)
	}
	if !strings.Contains(got, "公告如下") {
		t.Fatalf("body missing: %q", got)
	}
}

func TestAnnouncementExcerptStartsAtTitle(t *testing.T) {
	text := "首页 机构设置 湖南省2027年定向选调公告 首页 湖南省2027年定向选调公告 根据选调生工作的规定和要求，现就定向选调有关事项公告如下。"
	got := announcementExcerpt(text, "湖南省2027年定向选调公告")
	if !strings.HasPrefix(got, "湖南省2027年定向选调公告 根据选调生") {
		t.Fatalf("%q", got)
	}
}

func TestNextListPage(t *testing.T) {
	html := `<script>window.onload=function() {createPageHTML('paging',25, 1,'index','html',500);}</script>`
	next := nextListPage(html, "https://rst.hunan.gov.cn/rst/xxgk/zpzl/sydwzp/")
	if next != "https://rst.hunan.gov.cn/rst/xxgk/zpzl/sydwzp/index_2.html" {
		t.Fatalf("next %s", next)
	}
	if nextListPage(`createPageHTML('paging',25, 25,'index','html',500)`, "https://rst.hunan.gov.cn/rst/xxgk/zpzl/sydwzp/index_25.html") != "" {
		t.Fatal("expected last page to stop")
	}
}
