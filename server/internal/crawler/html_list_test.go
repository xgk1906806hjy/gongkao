package crawler

import "testing"

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
