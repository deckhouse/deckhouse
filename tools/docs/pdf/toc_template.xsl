<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="2.0"
                xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
                xmlns:outline="http://wkhtmltopdf.org/outline"
                xmlns="http://www.w3.org/1999/xhtml">
  <xsl:output doctype-public="-//W3C//DTD XHTML 1.0 Strict//EN"
              doctype-system="http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"
              indent="yes" />
  <xsl:template match="outline:outline">
    <html>
      <head>
        <title>Содержание</title>
        <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
        <style>
          h1 {
            text-align: center;
            font-size: 20px;
            font-family: arial;
          }
          li {list-style: none;}
          ul {
            font-size: 20px;
            font-family: arial;
            padding-left: 0em;
            margin: 0;
          }
          ul ul {
            font-size: 20px;
            padding-left: 1em;
          }
          a {
            text-decoration: none;
            color: black;
          }
          /* Используем table layout для правильного позиционирования */
          .toc-item {
            display: table;
            width: 100%;
            table-layout: fixed;
            border-bottom: 1px dashed rgb(200,200,200);
            margin-bottom: 0.2em;
          }
          .toc-link {
            display: table-cell;
            width: auto;
            padding-right: 1em;
            word-wrap: break-word;
            overflow-wrap: break-word;
            white-space: normal;
            vertical-align: top;
          }
          .toc-page {
            display: table-cell;
            width: 3.5em;
            min-width: 3.5em;
            max-width: 3.5em;
            text-align: right;
            vertical-align: top;
            white-space: nowrap;
            padding-left: 0.5em;
          }
        </style>
      </head>
      <body>
        <h1>Содержание</h1>
        <ul><xsl:apply-templates select="outline:item/outline:item"/></ul>
      </body>
    </html>
  </xsl:template>
  <xsl:template match="outline:item">
    <li>
      <xsl:if test="@title!=''">
        <div class="toc-item">
          <span class="toc-link">
            <a>
              <xsl:if test="@link">
                <xsl:attribute name="href"><xsl:value-of select="@link"/></xsl:attribute>
              </xsl:if>
              <xsl:if test="@backLink">
                <xsl:attribute name="name"><xsl:value-of select="@backLink"/></xsl:attribute>
              </xsl:if>
              <xsl:value-of select="@title" />
            </a>
          </span>
          <span class="toc-page">
            <xsl:value-of select="@page" />
          </span>
        </div>
      </xsl:if>
      <ul>
        <xsl:comment>added to prevent self-closing tags in QtXmlPatterns</xsl:comment>
        <xsl:apply-templates select="outline:item"/>
      </ul>
    </li>
  </xsl:template>
</xsl:stylesheet>
