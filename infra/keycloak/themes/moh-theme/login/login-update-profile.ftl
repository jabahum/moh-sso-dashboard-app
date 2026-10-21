<#import "template.ftl" as layout>

<@layout.registrationLayout displayMessage=true; section>
    <#if section = "form">
        <form id="kc-update-profile-form" class="moh-form" action="${url.loginAction}" method="post">
            <div class="moh-page-intro">
                <h3>${msg("updateProfileTitle")}</h3>
                <p>${msg("updateProfileInstruction")}</p>
            </div>

            <#if user.profile.attributes??>
                <#list user.profile.attributes as attribute>
                    <div class="moh-field">
                        <label for="${attribute.name}" class="moh-label">
                            ${advancedMsg(attribute.displayName!'')}
                            <#if attribute.required>
                                <span class="moh-required">*</span>
                            </#if>
                        </label>
                        <input
                            id="${attribute.name}"
                            name="${attribute.name}"
                            type="text"
                            class="moh-input"
                            value="${(attribute.value!'')}"
                            <#if attribute.readOnly>readonly</#if>
                        />
                    </div>
                </#list>
            <#else>
                <div class="moh-field">
                    <label for="firstName" class="moh-label">${msg("firstName")}</label>
                    <input id="firstName" name="firstName" type="text" class="moh-input" value="${(user.firstName!'')}" />
                </div>
                <div class="moh-field">
                    <label for="lastName" class="moh-label">${msg("lastName")}</label>
                    <input id="lastName" name="lastName" type="text" class="moh-input" value="${(user.lastName!'')}" />
                </div>
                <div class="moh-field">
                    <label for="email" class="moh-label">${msg("email")}</label>
                    <input id="email" name="email" type="email" class="moh-input" value="${(user.email!'')}" />
                </div>
            </#if>

            <div class="moh-actions">
                <button class="moh-primary-btn" type="submit">
                    ${msg("doSubmit")}
                </button>
            </div>
        </form>
    </#if>
</@layout.registrationLayout>
